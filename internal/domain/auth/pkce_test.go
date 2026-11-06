package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPKCE(t *testing.T) {
	t.Parallel()

	t.Run("new", func(t *testing.T) {
		t.Parallel()
		pk, err := NewPKCE()
		require.NoError(t, err)
		assert.Len(t, pk.Verifier, 64)
		for _, r := range pk.Verifier {
			assert.True(t, strings.ContainsRune(pkceAlphabet, r),
				"verifier has invalid character %q", string(r))
		}
		assert.NotEmpty(t, pk.Challenge, "challenge is empty")
		assert.NotEmpty(t, pk.State, "state is empty")
		other, err := NewPKCE()
		require.NoError(t, err)
		assert.NotEqual(t, pk.Verifier, other.Verifier, "PKCE values should be random")
		assert.NotEqual(t, pk.State, other.State, "PKCE values should be random")
	})

	t.Run("authorize url", func(t *testing.T) {
		t.Parallel()
		pk := PKCE{
			Verifier:  "verifier123",
			Challenge: "challenge123",
			State:     "state123",
		}
		got := pk.AuthorizeURL(
			"https://q.trap.jp",
			"client-id",
			"http://127.0.0.1:1234/callback",
			[]string{"read", "write"},
		)
		for _, want := range []string{
			"https://q.trap.jp/api/v3/oauth2/authorize?",
			"response_type=code",
			"client_id=client-id",
			"redirect_uri=http%3A%2F%2F127.0.0.1%3A1234%2Fcallback",
			"scope=read+write",
			"state=state123",
			"code_challenge=challenge123",
			"code_challenge_method=S256",
		} {
			assert.Contains(t, got, want)
		}
	})
}
