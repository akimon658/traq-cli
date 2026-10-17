package auth

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPKCE(t *testing.T) {
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
}

func TestAuthorizeURL(t *testing.T) {
	pk := PKCE{
		Verifier:  "verifier123",
		Challenge: "challenge123",
		State:     "state123",
	}
	got := AuthorizeURL(
		"https://q.trap.jp",
		"client-id",
		"http://127.0.0.1:1234/callback",
		[]string{"read", "write"},
		pk,
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
}

func TestCallbackServer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pk, err := NewPKCE()
		require.NoError(t, err)
		cb, err := ListenCallback(pk.State)
		require.NoError(t, err)
		defer cb.Close()
		redirectURI := cb.RedirectURI()
		assert.True(t, strings.HasPrefix(redirectURI, "http://127.0.0.1:"),
			"redirect URI = %q", redirectURI)
		_, err = httpGet(redirectURI + "?code=xyz&state=" + pk.State)
		require.NoError(t, err)
		code, err := cb.Wait(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "xyz", code)
	})

	t.Run("state mismatch", func(t *testing.T) {
		pk, err := NewPKCE()
		require.NoError(t, err)
		cb, err := ListenCallback(pk.State)
		require.NoError(t, err)
		defer cb.Close()
		_, err = httpGet(cb.RedirectURI() + "?code=xyz&state=wrong")
		require.NoError(t, err)
		_, err = cb.Wait(t.Context())
		assert.Error(t, err, "state mismatch should be an error")
	})

	t.Run("error parameter", func(t *testing.T) {
		pk, err := NewPKCE()
		require.NoError(t, err)
		cb, err := ListenCallback(pk.State)
		require.NoError(t, err)
		defer cb.Close()
		_, err = httpGet(cb.RedirectURI() + "?error=access_denied&state=" + pk.State)
		require.NoError(t, err)
		_, err = cb.Wait(t.Context())
		assert.Error(t, err, "error parameter should be an error")
	})
}

func httpGet(url string) (int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}
