package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/auth"
)

func TestNewCookieForRequestCreatesPersistentCookie(t *testing.T) {
	authenticator, err := auth.NewAuthenticator([]byte("01234567890123456789012345678901"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080", nil)
	cookie, err := authenticator.NewCookieForRequest("user-1", req)
	require.NoError(t, err)

	assert.Equal(t, "user_id", cookie.Name)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.False(t, cookie.Secure)
	assert.Equal(t, 365*24*60*60, cookie.MaxAge)
	assert.WithinDuration(t, time.Now().Add(365*24*time.Hour), cookie.Expires, 2*time.Second)
}

func TestNewCookieForRequestMarksHTTPSCookieAsSecure(t *testing.T) {
	authenticator, err := auth.NewAuthenticator([]byte("01234567890123456789012345678901"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	cookie, err := authenticator.NewCookieForRequest("user-1", req)
	require.NoError(t, err)

	assert.True(t, cookie.Secure)
}
