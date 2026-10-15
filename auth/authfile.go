package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nbigot/ministream/config"
	"go.uber.org/zap"
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
	}
	// unknown user: wrong credentials (this is not an internal error)
	return false, nil
}

// credentialsFromHashes converts a credentials document (user id -> hashed password)
// into PBKDF2 hashes. Keys starting with an underscore are comments (ex: "_WARNING")
// and are ignored.
func credentialsFromHashes(hashes map[string]string) (map[string]*Pbkdf2, error) {
	creds := map[string]*Pbkdf2{}
	for username, pbkdf2Hash := range hashes {
		if strings.HasPrefix(username, "_") {
			continue
		}
		pbkdf2, err := HashedPasswordToPbkdf2(pbkdf2Hash)
		if err != nil {
			return nil, fmt.Errorf("invalid hash for user %s: %w", username, err)
		}
		creds[username] = pbkdf2
	}
	return creds, nil
}

func (m *AuthFile) LoadCredentialsFromFile(filename string) error {
	if filename == "" {
		// empty credentials
		m.creds = map[string]*Pbkdf2{}
		return nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("can't open credential file: %s", filename)
	}
	defer func() {
		_ = file.Close()
	}()

	jsonDecoder := json.NewDecoder(file)
	strCreds := map[string]string{}
	if err := jsonDecoder.Decode(&strCreds); err != nil {
		return err
	}
	creds, err := credentialsFromHashes(strCreds)
	if err != nil {
		return err
	}
	m.creds = creds
	return nil
}

func (m *AuthFile) Initialize(logger *zap.Logger, c *config.AuthConfig) error {
	// example: "/app/data/secrets/secrets.json"
	if c.Methods.File.Filename == "" && c.Method == "FILE" && c.Enable {
		err := fmt.Errorf("filename is empty")
		logger.Error(
			"can't initialize credentials",
			zap.String("topic", "credentials"),
			zap.String("method", "Initialize"),
			zap.String("filename", ""),
			zap.Error(err),
		)
		return err
	}

	m.filename = c.Methods.File.Filename
	if err := m.LoadCredentialsFromFile(m.filename); err != nil {
		logger.Error(
			"can't initialize credentials",
			zap.String("topic", "credentials"),
			zap.String("method", "Initialize"),
			zap.String("filename", m.filename),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func init() {
	authFile = &AuthFile{}
	RegisterAuthenticationMethod(authFile)
}
