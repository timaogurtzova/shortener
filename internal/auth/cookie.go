package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const defaultCookieName = "user_id"
const cookieTTL = 365 * 24 * time.Hour

// ErrUserIDMissing возвращается, когда cookie корректно подписана, но не содержит user ID.
var ErrUserIDMissing = errors.New("user id is missing")

// ErrAuthorizationInvalid возвращается для некорректных авторизационных данных.
var ErrAuthorizationInvalid = errors.New("authorization is invalid")

type cookieState int

const (
	cookieStateValid cookieState = iota
	cookieStateMissing
	cookieStateInvalid
	cookieStateEmptyUserID
)

// Authenticator управляет подписью и валидацией пользовательской cookie.
type Authenticator struct {
	cookieName string
	secret     []byte
}

// NewAuthenticator создаёт вспомогательный объект для пользовательской cookie.
func NewAuthenticator(secret []byte) (*Authenticator, error) {
	if len(secret) == 0 {
		return nil, errors.New("auth secret is empty")
	}

	secretCopy := make([]byte, len(secret))
	copy(secretCopy, secret)

	return &Authenticator{
		cookieName: defaultCookieName,
		secret:     secretCopy,
	}, nil
}

// NewRandomSecret генерирует симметричный ключ для подписи cookie.
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

// EnsureUserID возвращает существующий user ID либо создаёт новый и устанавливает cookie.
func (a *Authenticator) EnsureUserID(w http.ResponseWriter, r *http.Request) (string, error) {
	userID, state, err := a.resolveUserID(r)
	if err != nil {
		return "", err
	}

	if state == cookieStateValid {
		return userID, nil
	}

	userID, err = generateUserID(16)
	if err != nil {
		return "", err
	}

	if err := a.setCookie(w, r, userID); err != nil {
		return "", err
	}

	return userID, nil
}

// UserIDForHistory возвращает user ID для запроса списка URL пользователя.
func (a *Authenticator) UserIDForHistory(w http.ResponseWriter, r *http.Request) (string, error) {
	userID, state, err := a.resolveUserID(r)
	if err != nil {
		return "", err
	}

	switch state {
	case cookieStateValid:
		return userID, nil
	case cookieStateMissing:
		return a.EnsureUserID(w, r)
	case cookieStateEmptyUserID:
		return "", ErrUserIDMissing
	default:
		return "", ErrUserIDMissing
	}
}

// UserID возвращает существующий валидный user ID, не создавая новую cookie.
func (a *Authenticator) UserID(r *http.Request) (string, bool, error) {
	userID, state, err := a.resolveUserID(r)
	if err != nil {
		return "", false, err
	}

	if state == cookieStateValid {
		return userID, true, nil
	}

	return "", false, nil
}

// EnsureAuthorization возвращает существующий user ID из authorization либо
// создаёт нового пользователя и возвращает новое подписанное значение.
func (a *Authenticator) EnsureAuthorization(authorization string) (userID, newAuthorization string, err error) {
	userID, state := a.resolveAuthorization(authorization)
	if state == cookieStateValid {
		return userID, "", nil
	}

	return a.newAuthorization()
}

// AuthorizationForHistory возвращает user ID для запроса истории.
// При отсутствии authorization создаёт нового пользователя, а некорректные
// данные отклоняет.
func (a *Authenticator) AuthorizationForHistory(authorization string) (userID, newAuthorization string, err error) {
	userID, state := a.resolveAuthorization(authorization)

	switch state {
	case cookieStateValid:
		return userID, "", nil
	case cookieStateMissing:
		return a.newAuthorization()
	default:
		return "", "", ErrAuthorizationInvalid
	}
}

// AuthorizationUserID возвращает существующий валидный user ID, не создавая нового.
func (a *Authenticator) AuthorizationUserID(authorization string) (string, bool) {
	userID, state := a.resolveAuthorization(authorization)
	return userID, state == cookieStateValid
}

func (a *Authenticator) resolveUserID(r *http.Request) (string, cookieState, error) {
	cookie, err := r.Cookie(a.cookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", cookieStateMissing, nil
		}
		return "", cookieStateInvalid, nil
	}

	userID, state := a.decode(cookie.Value)
	return userID, state, nil
}

func (a *Authenticator) resolveAuthorization(authorization string) (string, cookieState) {
	authorization = strings.TrimSpace(authorization)
	if authorization == "" {
		return "", cookieStateMissing
	}

	token := authorization
	fields := strings.Fields(authorization)
	if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
		token = fields[1]
	} else if strings.HasPrefix(token, a.cookieName+"=") {
		token = strings.TrimPrefix(token, a.cookieName+"=")
	}

	return a.decode(token)
}

func (a *Authenticator) newAuthorization() (userID, authorization string, err error) {
	userID, err = generateUserID(16)
	if err != nil {
		return "", "", err
	}

	authorization, err = a.encode(userID)
	if err != nil {
		return "", "", err
	}

	return userID, authorization, nil
}

func (a *Authenticator) setCookie(w http.ResponseWriter, r *http.Request, userID string) error {
	cookie, err := a.NewCookieForRequest(userID, r)
	if err != nil {
		return err
	}

	http.SetCookie(w, cookie)

	return nil
}

// NewCookie создаёт подписанную cookie для указанного user ID.
func (a *Authenticator) NewCookie(userID string) (*http.Cookie, error) {
	return a.newCookie(userID, false)
}

// NewCookieForRequest создаёт подписанную cookie для запроса с учётом защищённого транспорта.
func (a *Authenticator) NewCookieForRequest(userID string, r *http.Request) (*http.Cookie, error) {
	return a.newCookie(userID, isSecureRequest(r))
}

func (a *Authenticator) newCookie(userID string, secure bool) (*http.Cookie, error) {
	encodedValue, err := a.encode(userID)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(cookieTTL)

	return &http.Cookie{
		Name:     a.cookieName,
		Value:    encodedValue,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   int(cookieTTL.Seconds()),
		Expires:  expiresAt,
	}, nil
}

func (a *Authenticator) encode(userID string) (string, error) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(userID))

	signature, err := a.sign(payload)
	if err != nil {
		return "", err
	}

	return payload + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (a *Authenticator) decode(value string) (string, cookieState) {
	payload, signature, ok := strings.Cut(value, ".")
	if !ok {
		return "", cookieStateInvalid
	}

	expectedSignature, err := a.sign(payload)
	if err != nil {
		return "", cookieStateInvalid
	}

	actualSignature, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", cookieStateInvalid
	}

	if !hmac.Equal(actualSignature, expectedSignature) {
		return "", cookieStateInvalid
	}

	userIDBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", cookieStateInvalid
	}

	if len(userIDBytes) == 0 {
		return "", cookieStateEmptyUserID
	}

	return string(userIDBytes), cookieStateValid
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

func isSecureRequest(r *http.Request) bool {
	if r == nil {
		return false
	}

	if r.TLS != nil {
		return true
	}

	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}

	return strings.EqualFold(r.Header.Get("X-Forwarded-Ssl"), "on")
}
