package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akimon658/traq-cli/internal/auth"
)

func TestWriteAuthStatusBot(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, writeAuthStatus(&buf, "https://example.com", "environment", auth.Token{
		Method:   auth.MethodBot,
		UserID:   "user-id",
		UserName: "BOT_x",
		UserBot:  true,
	}))
	out := buf.String()
	assert.Contains(t, out, "origin: https://example.com")
	assert.Contains(t, out, "source: environment")
	assert.Contains(t, out, "method: bot")
	assert.Contains(t, out, "user: @BOT_x")
	assert.Contains(t, out, "userId: user-id")
	assert.NotContains(t, out, `"origin"`)
	assert.NotContains(t, out, "scopes:")
}

func TestWriteAuthStatusOAuth(t *testing.T) {
	t.Parallel()
	expires := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	require.NoError(t, writeAuthStatus(&buf, "https://example.com", "keychain", auth.Token{
		Method:    auth.MethodOAuth,
		Scope:     []string{auth.ScopeRead},
		UserID:    "user-id",
		UserName:  "alice",
		ExpiresAt: expires,
	}))
	out := buf.String()
	assert.Contains(t, out, "method: oauth")
	assert.Contains(t, out, "scopes: read")
	assert.Contains(t, out, "expiresAt: 2026-10-18T10:00:00Z")
}

func TestAuthStatusMethod(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "bot", authStatusMethod(&auth.Token{Method: auth.MethodBot, UserBot: true}))
	assert.Equal(t, "oauth", authStatusMethod(&auth.Token{Method: auth.MethodOAuth}))
	assert.Equal(t, "", authStatusMethod(&auth.Token{}))
}
