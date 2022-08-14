package auth

import (
	"encoding/json"
	"fmt"
	"ministream/config"
	"os"
)

type AuthFile struct {
	filename string
	creds    map[string]*Pbkdf2
}

var authFile *AuthFile

func (m *AuthFile) GetMethodName() string {
	return "FILE"
}

func (m *AuthFile) AuthenticateUser(userId string, password string) (bool, error) {
	if pbkdf2, foundUser := m.creds[userId]; foundUser {
		return pbkdf2.Verify(password)
	} else {
		return false, fmt.Errorf("user id not found: %s", userId)
	}
}

func (m *AuthFile) LoadCredentialsFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonDecoder := json.NewDecoder(file)
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
	return nil
}

func (m *AuthFile) Initialize(c *config.Config) error {
	if c.Auth.Methods.File.Filename == "" {
		m.filename = c.DataDirectory + "secrets.json"
	} else {
		m.filename = c.Auth.Methods.File.Filename
	}
	return m.LoadCredentialsFromFile(m.filename)
}

func init() {
	authFile = &AuthFile{}
	RegisterAuthenticationMethod(authFile)
}
