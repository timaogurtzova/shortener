package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

const defaultCookieName = "user_id"
const cookieTTL = 365 * 24 * time.Hour

// ErrUserIDMissing возвращается, когда cookie корректно подписана, но не содержит user ID.
var ErrUserIDMissing = errors.New("user id is missing")

// EnsureUserID возвращает существующий user ID либо создаёт новый и устанавливает cookie.
func (a *Authenticator) EnsureUserID(w http.ResponseWriter, r *http.Request) (string, error) {
	userID, state, err := a.resolveUserID(r)
	if err != nil {
		return "", err
	}

	if state == credentialStateValid {
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
	case credentialStateValid:
		return userID, nil
	case credentialStateMissing:
		return a.EnsureUserID(w, r)
	case credentialStateEmptyUserID:
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

	if state == credentialStateValid {
		return userID, true, nil
	}

	return "", false, nil
}

func (a *Authenticator) resolveUserID(r *http.Request) (string, credentialState, error) {
	cookie, err := r.Cookie(defaultCookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", credentialStateMissing, nil
		}
		return "", credentialStateInvalid, nil
	}

	userID, state := a.decode(cookie.Value)
	return userID, state, nil
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
		Name:     defaultCookieName,
		Value:    encodedValue,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   int(cookieTTL.Seconds()),
		Expires:  expiresAt,
	}, nil
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
