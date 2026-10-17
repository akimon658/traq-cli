package auth

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewPKCE(t *testing.T) {
	pk, err := NewPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Verifier) != 64 {
		t.Errorf("verifier length = %d, want 64", len(pk.Verifier))
	}
	for _, r := range pk.Verifier {
		if !strings.ContainsRune(pkceAlphabet, r) {
			t.Errorf("verifier has invalid character %q", string(r))
		}
	}
	if len(pk.Challenge) == 0 {
		t.Error("challenge is empty")
	}
	if len(pk.State) == 0 {
		t.Error("state is empty")
	}
	other, _ := NewPKCE()
	if pk.Verifier == other.Verifier || pk.State == other.State {
		t.Error("PKCE values should be random")
	}
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
		if !strings.Contains(got, want) {
			t.Errorf("AuthorizeURL = %q, want to contain %q", got, want)
		}
	}
}

func TestCallbackServer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pk, err := NewPKCE()
		if err != nil {
			t.Fatal(err)
		}
		cb, err := ListenCallback(pk.State)
		if err != nil {
			t.Fatal(err)
		}
		defer cb.Close()
		redirectURI := cb.RedirectURI()
		if !strings.HasPrefix(redirectURI, "http://127.0.0.1:") {
			t.Errorf("redirect URI = %q", redirectURI)
		}
		if _, err := httpGet(redirectURI + "?code=xyz&state=" + pk.State); err != nil {
			t.Fatal(err)
		}
		code, err := cb.Wait(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if code != "xyz" {
			t.Errorf("code = %q, want xyz", code)
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		pk, err := NewPKCE()
		if err != nil {
			t.Fatal(err)
		}
		cb, err := ListenCallback(pk.State)
		if err != nil {
			t.Fatal(err)
		}
		defer cb.Close()
		if _, err := httpGet(cb.RedirectURI() + "?code=xyz&state=wrong"); err != nil {
			t.Fatal(err)
		}
		if _, err := cb.Wait(t.Context()); err == nil {
			t.Error("state mismatch should be an error")
		}
	})

	t.Run("error parameter", func(t *testing.T) {
		pk, err := NewPKCE()
		if err != nil {
			t.Fatal(err)
		}
		cb, err := ListenCallback(pk.State)
		if err != nil {
			t.Fatal(err)
		}
		defer cb.Close()
		if _, err := httpGet(
			cb.RedirectURI() + "?error=access_denied&state=" + pk.State,
		); err != nil {
			t.Fatal(err)
		}
		_, err = cb.Wait(t.Context())
		if err == nil {
			t.Error("error parameter should be an error")
		}
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
