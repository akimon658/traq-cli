package cli

import (
	"errors"
	"testing"
	"time"

	"github.com/akimon658/traq-cli/internal/auth"
)

type fakeStore struct {
	token   auth.Token
	source  string
	found   bool
	loadErr error

	saves   int
	loads   int
	deletes int
}

func (f *fakeStore) Save(t auth.Token) (string, error) {
	f.saves++
	f.token = t
	return f.source, nil
}

func (f *fakeStore) Load() (auth.Token, string, bool, error) {
	f.loads++
	if f.loadErr != nil {
		return auth.Token{}, "", false, f.loadErr
	}
	return f.token, f.source, f.found, nil
}

func (f *fakeStore) Delete() (bool, error) {
	f.deletes++
	f.found = false
	return true, nil
}

func TestResolveTokenPrefersEnvironment(t *testing.T) {
	t.Setenv("TRAQ_ACCESS_TOKEN", "env-token")
	fs := &fakeStore{}
	e := &Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: fs}
	rt, err := e.resolveToken()
	if err != nil {
		t.Fatal(err)
	}
	if rt.Source != "environment" {
		t.Errorf("source = %q, want %q", rt.Source, "environment")
	}
	if rt.Token.AccessToken != "env-token" {
		t.Errorf("accessToken = %q, want %q", rt.Token.AccessToken, "env-token")
	}
	if fs.loads != 0 {
		t.Errorf("store.Load called %d times, want 0", fs.loads)
	}
}

func TestResolveTokenPrefersFlag(t *testing.T) {
	t.Setenv("TRAQ_ACCESS_TOKEN", "env-token")
	oldFlagToken := flagToken
	flagToken = "flag-token"
	t.Cleanup(func() { flagToken = oldFlagToken })

	e := &Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: &fakeStore{}}
	rt, err := e.resolveToken()
	if err != nil {
		t.Fatal(err)
	}
	if rt.Source != "flag" {
		t.Errorf("source = %q, want %q", rt.Source, "flag")
	}
	if rt.Token.AccessToken != "flag-token" {
		t.Errorf("accessToken = %q, want %q", rt.Token.AccessToken, "flag-token")
	}
}

func TestResolveTokenFromStore(t *testing.T) {
	fs := &fakeStore{
		token: auth.Token{
			Method:      auth.MethodOAuth,
			AccessToken: "stored-token",
			Scope:       []string{auth.ScopeRead},
			Origin:      auth.DefaultOrigin,
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		},
		source: "keychain",
		found:  true,
	}
	e := &Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: fs}
	rt, err := e.resolveToken()
	if err != nil {
		t.Fatal(err)
	}
	if rt.Source != "keychain" {
		t.Errorf("source = %q, want %q", rt.Source, "keychain")
	}
	if rt.Token.AccessToken != "stored-token" {
		t.Errorf("accessToken = %q, want %q", rt.Token.AccessToken, "stored-token")
	}
	if fs.loads != 1 {
		t.Errorf("store.Load called %d times, want 1", fs.loads)
	}
}

func TestResolveTokenNotAuthenticated(t *testing.T) {
	fs := &fakeStore{}
	e := &Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: fs}
	_, err := e.resolveToken()
	var ec *ExitCodeError
	if !errors.As(err, &ec) {
		t.Fatalf("err = %v, want ExitCodeError", err)
	}
	if ec.Code != exitAuthRequired {
		t.Errorf("code = %d, want %d", ec.Code, exitAuthRequired)
	}
}

func TestResolveTokenExpiredWithoutRefreshToken(t *testing.T) {
	fs := &fakeStore{
		token: auth.Token{
			Method:      auth.MethodOAuth,
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour),
		},
		source: "keychain",
		found:  true,
	}
	e := &Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: fs}
	_, err := e.resolveToken()
	var ec *ExitCodeError
	if !errors.As(err, &ec) {
		t.Fatalf("err = %v, want ExitCodeError", err)
	}
	if ec.Code != exitAuthRequired {
		t.Errorf("code = %d, want %d", ec.Code, exitAuthRequired)
	}
	if fs.deletes != 0 {
		t.Errorf("store.Delete called %d times, want 0", fs.deletes)
	}
}
