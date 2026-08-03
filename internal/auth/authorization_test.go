package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/auth"
)

func TestAuthorizationLifecycle(t *testing.T) {
	authenticator, err := auth.NewAuthenticator([]byte("01234567890123456789012345678901"))
	require.NoError(t, err)

	userID, authorization, err := authenticator.EnsureAuthorization("")
	require.NoError(t, err)
	assert.NotEmpty(t, userID)
	assert.NotEmpty(t, authorization)

	existingUserID, newAuthorization, err := authenticator.EnsureAuthorization("Bearer " + authorization)
	require.NoError(t, err)
	assert.Equal(t, userID, existingUserID)
	assert.Empty(t, newAuthorization)

	historyUserID, newAuthorization, err := authenticator.AuthorizationForHistory(authorization)
	require.NoError(t, err)
	assert.Equal(t, userID, historyUserID)
	assert.Empty(t, newAuthorization)

	resolvedUserID, ok := authenticator.AuthorizationUserID(authorization)
	assert.True(t, ok)
	assert.Equal(t, userID, resolvedUserID)
}

func TestAuthorizationRejectsInvalidFormats(t *testing.T) {
	authenticator, err := auth.NewAuthenticator([]byte("01234567890123456789012345678901"))
	require.NoError(t, err)

	_, authorization, err := authenticator.EnsureAuthorization("")
	require.NoError(t, err)

	tests := []string{
		"broken",
		"Basic " + authorization,
		"Bearer " + authorization + " extra",
		"user_id=" + authorization,
	}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			userID, newAuthorization, historyErr := authenticator.AuthorizationForHistory(value)

			require.ErrorIs(t, historyErr, auth.ErrAuthorizationInvalid)
			assert.Empty(t, userID)
			assert.Empty(t, newAuthorization)
		})
	}
}
