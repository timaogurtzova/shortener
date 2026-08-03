package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"math/big"
	"strings"
)

type credentialState int

const (
	credentialStateValid credentialState = iota
	credentialStateMissing
	credentialStateInvalid
	credentialStateEmptyUserID
)

// Authenticator подписывает и проверяет пользовательские учётные данные.
type Authenticator struct {
	secret []byte
}

// NewAuthenticator создаёт аутентификатор пользовательских учётных данных.
func NewAuthenticator(secret []byte) (*Authenticator, error) {
	if len(secret) == 0 {
		return nil, errors.New("auth secret is empty")
	}

	secretCopy := make([]byte, len(secret))
	copy(secretCopy, secret)

	return &Authenticator{secret: secretCopy}, nil
}

// NewRandomSecret генерирует симметричный ключ для подписи учётных данных.
func NewRandomSecret(size int) ([]byte, error) {
	if size <= 0 {
		return nil, errors.New("invalid secret size")
	}

	secret := make([]byte, size)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func (a *Authenticator) encode(userID string) (string, error) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(userID))

	signature, err := a.sign(payload)
	if err != nil {
		return "", err
	}

	return payload + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (a *Authenticator) decode(value string) (string, credentialState) {
	payload, signature, ok := strings.Cut(value, ".")
	if !ok {
		return "", credentialStateInvalid
	}

	expectedSignature, err := a.sign(payload)
	if err != nil {
		return "", credentialStateInvalid
	}

	actualSignature, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", credentialStateInvalid
	}

	if !hmac.Equal(actualSignature, expectedSignature) {
		return "", credentialStateInvalid
	}

	userIDBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", credentialStateInvalid
	}

	if len(userIDBytes) == 0 {
		return "", credentialStateEmptyUserID
	}

	return string(userIDBytes), credentialStateValid
}

func (a *Authenticator) sign(payload string) ([]byte, error) {
	mac := hmac.New(sha256.New, a.secret)
	if _, err := mac.Write([]byte(payload)); err != nil {
		return nil, err
	}

	return mac.Sum(nil), nil
}

func generateUserID(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	var result strings.Builder
	result.Grow(length)

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}

		result.WriteByte(alphabet[num.Int64()])
	}

	return result.String(), nil
}
