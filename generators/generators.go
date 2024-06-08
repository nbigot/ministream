package generators

import (
	"math/rand"
	"strings"
	"time"
)

const SaltChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
const PasswordChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-."
const SecretAPIKeyChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func GenerateRandomSalt(length int, initSeed bool) string {
	return GenerateRandomString(length, []rune(SaltChars), initSeed)
}

func GenerateRandomPassword(length int, initSeed bool) string {
	chars := []rune(PasswordChars)
	return GenerateRandomString(length, chars, initSeed)
}

func GenerateRandomSecretAPIKey(length int) string {
	chars := []rune(SecretAPIKeyChars)
	return GenerateRandomString(length, chars, true)
}

func GenerateRandomString(length int, chars []rune, initSeed bool) string {
	if initSeed {
		rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}
