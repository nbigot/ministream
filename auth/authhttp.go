package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/nbigot/ministream/config"
	"go.uber.org/zap"
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
	creds, err := m.getCredentials()
	if err != nil {
		return false, err
	}

	if pbkdf2, foundUser := creds[userId]; foundUser {
		return pbkdf2.Verify(password)
	}
	// unknown user: wrong credentials (this is not an internal error)
	return false, nil
}

// getCredentials returns the credentials, they are loaded at the first authentication
// then reloaded when the cache has expired (when the cache duration is 0, the credentials
// are loaded for every authentication).
func (m *AuthHTTP) getCredentials() (map[string]*Pbkdf2, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.creds == nil || m.cacheDurationInSec <= 0 || time.Now().Unix() >= m.cacheExpiresAt {
		if err := m.loadCredentialsFromHTTP(); err != nil {
			return nil, err
		}
	}
	return m.creds, nil
}

func (m *AuthHTTP) LoadCredentialsFromHTTP() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadCredentialsFromHTTP()
}

func (m *AuthHTTP) loadCredentialsFromHTTP() error {
	apiUrl, err := url.Parse(m.url)
	if err != nil {
		return err
	}

	// note: the transport must stay a nil interface when there is no proxy
	// (a nil *http.Transport in the interface makes client.Do panic)
	client := &http.Client{Timeout: time.Second * time.Duration(m.timeoutInSec)}
	if m.proxyUrl != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(m.proxyUrl)}
	}

	r, _ := http.NewRequest(http.MethodGet, apiUrl.String(), nil)
	if m.authToken != "" {
		r.Header.Add("Authorization", fmt.Sprintf("auth_token=\"%s\"", m.authToken))
	}

	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode > 299 {
		return fmt.Errorf("LoadCredentialsFromHTTP: invalid response code %d", resp.StatusCode)
	}

	jsonDecoder := json.NewDecoder(resp.Body)

	strCreds := map[string]string{}
	if err := jsonDecoder.Decode(&strCreds); err != nil {
		return err
	}
	creds, err := credentialsFromHashes(strCreds)
	if err != nil {
		return err
	}
	m.creds = creds

	if m.cacheDurationInSec > 0 {
		m.cacheExpiresAt = time.Now().Add(time.Second * time.Duration(m.cacheDurationInSec)).Unix()
	}

	return nil
}

func (m *AuthHTTP) Initialize(logger *zap.Logger, c *config.AuthConfig) error {
	var err error
	m.url = c.Methods.HTTP.Url
	if c.Methods.HTTP.ProxyUrl != "" {
		if m.proxyUrl, err = url.Parse(c.Methods.HTTP.ProxyUrl); err != nil {
			return err
		}
	} else {
		m.proxyUrl = nil
	}
	m.timeoutInSec = c.Methods.HTTP.Timeout
	if m.timeoutInSec < 1 || m.timeoutInSec > 60 {
		m.timeoutInSec = 60
	}
	m.authToken = c.Methods.HTTP.AuthToken
	m.cacheDurationInSec = c.Methods.HTTP.CacheDurationInSec
	m.cacheExpiresAt = time.Now().Unix()
	// forget the credentials of a previous configuration (server restart)
	m.mu.Lock()
	m.creds = nil
	m.mu.Unlock()
	return nil
}

func init() {
	authHTTP = &AuthHTTP{}
	RegisterAuthenticationMethod(authHTTP)
}
