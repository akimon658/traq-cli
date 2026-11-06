package traq

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallbackServer(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		cb, err := ListenCallback("test-state")
		require.NoError(t, err)
		defer cb.Close()
		redirectURI := cb.RedirectURI()
		assert.True(t, strings.HasPrefix(redirectURI, "http://127.0.0.1:"),
			"redirect URI = %q", redirectURI)
		_, err = httpGet(redirectURI + "?code=xyz&state=test-state")
		require.NoError(t, err)
		code, err := cb.Wait(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "xyz", code)
	})

	t.Run("state mismatch", func(t *testing.T) {
		t.Parallel()
		cb, err := ListenCallback("test-state")
		require.NoError(t, err)
		defer cb.Close()
		_, err = httpGet(cb.RedirectURI() + "?code=xyz&state=wrong")
		require.NoError(t, err)
		_, err = cb.Wait(t.Context())
		assert.Error(t, err, "state mismatch should be an error")
	})

	t.Run("error parameter", func(t *testing.T) {
		t.Parallel()
		cb, err := ListenCallback("test-state")
		require.NoError(t, err)
		defer cb.Close()
		_, err = httpGet(cb.RedirectURI() + "?error=access_denied&state=test-state")
		require.NoError(t, err)
		_, err = cb.Wait(t.Context())
		assert.Error(t, err, "error parameter should be an error")
	})
}

// httpGet は ListenCallback が起動するローカルコールバックサーバーへ GET する。
// traQ API ではなく traq-cli 内の net/http サーバーが対象のため go-traq は使えない。
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
