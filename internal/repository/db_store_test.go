package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDBStore(t *testing.T) {
	store, err := NewDBStore(nil)
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Equal(t, "database is not configured", err.Error())
}
