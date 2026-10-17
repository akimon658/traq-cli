package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err)
	assert.Equal(t, "environment", rt.Source)
	assert.Equal(t, "env-token", rt.Token.AccessToken)
	assert.Zero(t, fs.loads, "store.Load called %d times, want 0", fs.loads)
}

func TestResolveTokenPrefersFlag(t *testing.T) {
	t.Setenv("TRAQ_ACCESS_TOKEN", "env-token")
	oldFlagToken := flagToken
	flagToken = "flag-token"
	t.Cleanup(func() { flagToken = oldFlagToken })

	e := &Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: &fakeStore{}}
	rt, err := e.resolveToken()
	require.NoError(t, err)
	assert.Equal(t, "flag", rt.Source)
	assert.Equal(t, "flag-token", rt.Token.AccessToken)
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
	require.NoError(t, err)
	assert.Equal(t, "keychain", rt.Source)
	assert.Equal(t, "stored-token", rt.Token.AccessToken)
	assert.Equal(t, 1, fs.loads, "store.Load called %d times, want 1", fs.loads)
}

func TestResolveTokenNotAuthenticated(t *testing.T) {
	fs := &fakeStore{}
	e := &Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: fs}
	_, err := e.resolveToken()
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitAuthRequired, ec.Code)
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
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitAuthRequired, ec.Code)
	assert.Zero(t, fs.deletes, "store.Delete called %d times, want 0", fs.deletes)
}
