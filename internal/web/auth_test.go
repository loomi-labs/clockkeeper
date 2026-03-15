package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("mypassword")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.True(t, CheckPassword("mypassword", hash))
}

func TestCheckPassword_Wrong(t *testing.T) {
	hash, err := HashPassword("correct")
	require.NoError(t, err)
	assert.False(t, CheckPassword("wrong", hash))
}

func TestIssueToken_Valid(t *testing.T) {
	auth := NewAuthInterceptor("test-secret-key")

	token, err := auth.IssueToken("alice")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	_, err = auth.validate("Bearer " + token)
	assert.NoError(t, err)
}

func TestIssueToken_WrongSecret(t *testing.T) {
	issuer := NewAuthInterceptor("secret-a")
	validator := NewAuthInterceptor("secret-b")

	token, err := issuer.IssueToken("alice")
	require.NoError(t, err)

	_, err = validator.validate("Bearer " + token)
	assert.Error(t, err)
}

func TestValidate_MissingHeader(t *testing.T) {
	auth := NewAuthInterceptor("secret")

	_, err := auth.validate("")
	assert.Error(t, err)

	_, err = auth.validate("NotBearer token")
	assert.Error(t, err)
}
