package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nbigot/ministream/account"
	"github.com/nbigot/ministream/config"
)

type AuthHTTP struct {
	url                string
	proxyUrl           *url.URL
	authToken          string
	cacheDurationInSec int
	timeoutInSec       int
	cacheExpiresAt     int64
	creds              map[string]*Pbkdf2
	mu                 sync.Mutex
}

var authHTTP *AuthHTTP

func (m *AuthHTTP) GetMethodName() string {
	return "HTTP"
}

func (m *AuthHTTP) AuthenticateUser(userId string, password string) (bool, error) {
	if m.cacheDurationInSec > 0 && m.cacheExpiresAt < time.Now().Unix() {
		if err := m.LoadCredentialsFromHTTP(); err != nil {
			return false, err
		}
	}

	if pbkdf2, foundUser := m.creds[userId]; foundUser {
		return pbkdf2.Verify(password)
	} else {
		return false, fmt.Errorf("user id not found: %s", userId)
	}
}

func (m *AuthHTTP) LoadCredentialsFromHTTP() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	strApiUrl := strings.Replace(m.url, "{accountid}", account.GetAccount().Id.String(), -1)
	apiUrl, err := url.Parse(strApiUrl)
	if err != nil {
		return err
	}

	var transport *http.Transport = nil
	if m.proxyUrl != nil {
		transport = &http.Transport{Proxy: http.ProxyURL(m.proxyUrl)}
	}
	client := &http.Client{Timeout: time.Second * time.Duration(m.timeoutInSec), Transport: transport}

	r, _ := http.NewRequest(http.MethodGet, apiUrl.String(), nil)
	if m.authToken != "" {
		r.Header.Add("Authorization", fmt.Sprintf("auth_token=\"%s\"", m.authToken))
	}

	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	if resp.StatusCode > 299 {
		return fmt.Errorf("LoadCredentialsFromHTTP: invalid response code %d", resp.StatusCode)
	}

	jsonDecoder := json.NewDecoder(resp.Body)

	strCreds := map[string]string{}
	if err := jsonDecoder.Decode(&strCreds); err != nil {
		return err
	}
	m.creds = map[string]*Pbkdf2{}
	for username, pbkdf2Hash := range strCreds {
		if pbkdf2, err := HashedPasswordToPbkdf2(pbkdf2Hash); err != nil {
			return err
		} else {
			m.creds[username] = pbkdf2
		}
	}

	if m.cacheDurationInSec > 0 {
		m.cacheExpiresAt = time.Now().Add(time.Second * time.Duration(m.cacheDurationInSec)).Unix()
	}

	return nil
}

func (m *AuthHTTP) Initialize(c *config.Config) error {
	var err error
	m.url = c.Auth.Methods.HTTP.Url
	if c.Auth.Methods.HTTP.ProxyUrl != "" {
		if m.proxyUrl, err = url.Parse(c.Auth.Methods.HTTP.ProxyUrl); err != nil {
			return err
		}
	} else {
		m.proxyUrl = nil
	}
	m.timeoutInSec = c.Auth.Methods.HTTP.Timeout
	if m.timeoutInSec < 1 || m.timeoutInSec > 60 {
		m.timeoutInSec = 60
	}
	m.authToken = c.Auth.Methods.HTTP.AuthToken
	m.cacheDurationInSec = c.Auth.Methods.HTTP.CacheDurationInSec
	m.cacheExpiresAt = time.Now().Unix()
	return nil
}

func init() {
	authHTTP = &AuthHTTP{}
	RegisterAuthenticationMethod(authHTTP)
}
