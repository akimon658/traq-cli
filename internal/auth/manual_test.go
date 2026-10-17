package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSessionStore(t *testing.T) SessionStore {
	t.Helper()
	dir := t.TempDir()
	return defaultSessionStore{userConfigDir: func() (string, error) {
		return dir, nil
	}}
}

func testManualSession() ManualSession {
	return ManualSession{
		Verifier:    "verifier-verifier-verifier-verifier-verifier",
		State:       "state-state-state-state-state",
		RedirectURI: ManualRedirectURI,
		Origin:      "https://q.trap.jp",
		ClientID:    "client-id",
		Scopes:      []string{ScopeRead},
		CreatedAt:   time.Now(),
	}
}

func TestManualSessionRoundTrip(t *testing.T) {
	t.Parallel()
	ss := newTestSessionStore(t)
	now := time.Now()

	want := testManualSession()
	path, err := ss.Save(want)
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path))

	got, found, err := ss.Load(now)
	require.NoError(t, err)
	assert.True(t, found)
	// JSON 往復で monotonic clock が落ちるため、CreatedAt は時刻として比較する
	assert.WithinDuration(t, want.CreatedAt, got.CreatedAt, 0)
	assert.Equal(t, want.Verifier, got.Verifier)
	assert.Equal(t, want.State, got.State)
	assert.Equal(t, want.RedirectURI, got.RedirectURI)
	assert.Equal(t, want.Origin, got.Origin)
	assert.Equal(t, want.ClientID, got.ClientID)
	assert.Equal(t, want.Scopes, got.Scopes)

	found, err = ss.Delete()
	require.NoError(t, err)
	assert.True(t, found)

	_, found, err = ss.Load(now)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestManualSessionMissing(t *testing.T) {
	t.Parallel()
	ss := newTestSessionStore(t)

	_, found, err := ss.Load(time.Now())
	require.NoError(t, err)
	assert.False(t, found)

	found, err = ss.Delete()
	require.NoError(t, err)
	assert.False(t, found)
}

func TestManualSessionExpired(t *testing.T) {
	t.Parallel()
	ss := newTestSessionStore(t)

	sess := testManualSession()
	sess.CreatedAt = time.Now().Add(-ManualSessionTTL - time.Minute)
	_, err := ss.Save(sess)
	require.NoError(t, err)

	_, found, err := ss.Load(time.Now())
	require.NoError(t, err)
	assert.False(t, found, "expired session should be removed")

	// 期限切れセッションは Load 時に削除される
	_, found, err = ss.Load(time.Now())
	require.NoError(t, err)
	assert.False(t, found)
}

func TestManualSessionNotYetExpired(t *testing.T) {
	t.Parallel()
	ss := newTestSessionStore(t)

	sess := testManualSession()
	sess.CreatedAt = time.Now().Add(-ManualSessionTTL + time.Minute)
	_, err := ss.Save(sess)
	require.NoError(t, err)

	_, found, err := ss.Load(time.Now())
	require.NoError(t, err)
	assert.True(t, found)
}

func TestManualSessionBrokenFile(t *testing.T) {
	t.Parallel()
	ss := newTestSessionStore(t)
	path, err := ss.Save(testManualSession())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("{broken"), credentialsFilePerm))

	_, _, err = ss.Load(time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run 'traq auth login --manual' again")

	_, found, lerr := ss.Load(time.Now())
	require.NoError(t, lerr)
	assert.False(t, found, "broken session should be removed")
}

func TestManualSessionRejectsIncomplete(t *testing.T) {
	t.Parallel()
	ss := newTestSessionStore(t)

	noVerifier := testManualSession()
	noVerifier.Verifier = ""
	_, err := ss.Save(noVerifier)
	require.Error(t, err)

	noState := testManualSession()
	noState.State = ""
	_, err = ss.Save(noState)
	require.Error(t, err)
}

func TestManualSessionFilePerm(t *testing.T) {
	t.Parallel()
	ss := newTestSessionStore(t)

	path, err := ss.Save(testManualSession())
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(credentialsFilePerm), info.Mode().Perm())
}
