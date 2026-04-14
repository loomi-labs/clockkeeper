package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueToken_Valid(t *testing.T) {
	auth := NewAuthInterceptor("test-secret-key")

	token, err := auth.IssueToken(42, false)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	userID, isAnon, err := auth.validate("Bearer " + token)
	assert.NoError(t, err)
	assert.Equal(t, 42, userID)
	assert.False(t, isAnon)
}

func TestIssueToken_Anonymous(t *testing.T) {
	auth := NewAuthInterceptor("test-secret-key")

	token, err := auth.IssueToken(99, true)
	require.NoError(t, err)

	userID, isAnon, err := auth.validate("Bearer " + token)
	assert.NoError(t, err)
	assert.Equal(t, 99, userID)
	assert.True(t, isAnon)
}

func TestIssueToken_WrongSecret(t *testing.T) {
	issuer := NewAuthInterceptor("secret-a")
	validator := NewAuthInterceptor("secret-b")

	token, err := issuer.IssueToken(42, false)
	require.NoError(t, err)

	_, _, err = validator.validate("Bearer " + token)
	assert.Error(t, err)
}

func TestValidate_MissingHeader(t *testing.T) {
	auth := NewAuthInterceptor("secret")

	_, _, err := auth.validate("")
	assert.Error(t, err)

	_, _, err = auth.validate("NotBearer token")
	assert.Error(t, err)
}
