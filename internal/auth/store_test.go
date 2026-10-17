package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/99designs/keyring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func testStore(t *testing.T, kr keyring.Keyring, openErr error) defaultStore {
	t.Helper()
	dir := t.TempDir()
	return defaultStore{
		openKeyring: func() (keyring.Keyring, error) {
			return kr, openErr
		},
		userConfigDir: func() (string, error) {
			return dir, nil
		},
	}
}

func testToken() Token {
	return Token{
		Method:       MethodOAuth,
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Scope:        []string{ScopeRead, ScopeWrite},
		Origin:       DefaultOrigin,
		UserID:       "00000000-0000-0000-0000-000000000000",
		UserName:     "akimo",
		UserBot:      true,
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
}

func assertRoundTrip(t *testing.T, s defaultStore, wantSource string, verify func(t *testing.T)) {
	t.Helper()
	orig := testToken()
	venue, err := s.Save(orig)
	require.NoError(t, err)
	assert.Equal(t, wantSource, venue)

	t.Cleanup(func() {
		_, err := s.Delete()
		assert.NoError(t, err, "failed to clean up")
	})

	loaded, source, found, err := s.Load()
	require.NoError(t, err)
	require.True(t, found, "token not found after save")
	assert.Equal(t, wantSource, source)
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"method", loaded.Method, orig.Method},
		{"accessToken", loaded.AccessToken, orig.AccessToken},
		{"refreshToken", loaded.RefreshToken, orig.RefreshToken},
		{"origin", loaded.Origin, orig.Origin},
		{"userId", loaded.UserID, orig.UserID},
		{"userName", loaded.UserName, orig.UserName},
		{"userBot", loaded.UserBot, orig.UserBot},
	}
	for _, c := range checks {
		assert.Equal(t, c.want, c.got, c.name)
	}
	assert.Equal(t, orig.Scope, loaded.Scope)
	assert.True(t, loaded.ExpiresAt.Equal(orig.ExpiresAt),
		"expiresAt = %v, want %v", loaded.ExpiresAt, orig.ExpiresAt)

	_, err = s.Save(orig)
	require.NoError(t, err)
	loaded2, _, found, err := s.Load()
	require.NoError(t, err)
	require.True(t, found, "reload failed")
	assert.Equal(t, orig.AccessToken, loaded2.AccessToken)

	if verify != nil {
		verify(t)
	}

	foundAfterDelete, err := s.Delete()
	require.NoError(t, err)
	assert.True(t, foundAfterDelete, "Delete should report the item was found")
	_, _, found, _ = s.Load()
	assert.False(t, found, "token should be gone after delete")
}

func TestDefaultStoreRoundTripViaKeyring(t *testing.T) {
	t.Parallel()
	kr := NewMockKeyring(gomock.NewController(t))
	var stored []byte
	setItem := func(item keyring.Item) error {
		stored = item.Data
		return nil
	}
	getItem := func(string) (keyring.Item, error) {
		if stored == nil {
			return keyring.Item{}, keyring.ErrKeyNotFound
		}
		return keyring.Item{Key: storeItem, Data: stored}, nil
	}
	removeItem := func(string) error {
		stored = nil
		return nil
	}
	gomock.InOrder(
		kr.EXPECT().Set(gomock.Any()).DoAndReturn(setItem).Times(1),
		kr.EXPECT().Get(storeItem).DoAndReturn(getItem).Times(1),
		kr.EXPECT().Set(gomock.Any()).DoAndReturn(setItem).Times(1),
		kr.EXPECT().Get(storeItem).DoAndReturn(getItem).Times(1),
		kr.EXPECT().Get(storeItem).DoAndReturn(getItem).Times(1),
		kr.EXPECT().Remove(storeItem).DoAndReturn(removeItem).Times(1),
		kr.EXPECT().Get(storeItem).DoAndReturn(getItem).Times(1),
		kr.EXPECT().Get(storeItem).DoAndReturn(getItem).Times(1),
		kr.EXPECT().Remove(storeItem).DoAndReturn(removeItem).Times(1),
	)
	s := testStore(t, kr, nil)
	path, err := s.filePath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), credentialsDirPerm))
	require.NoError(t, os.WriteFile(path, []byte("stale"), credentialsFilePerm))
	assertRoundTrip(t, s, storeSourceKeychain, nil)
	_, err = os.Stat(path)
	assert.ErrorIs(t, err, os.ErrNotExist,
		"stale credentials file should be removed after keyring save")
}

func TestDefaultStoreFileFallback(t *testing.T) {
	t.Parallel()
	s := testStore(t, nil, keyring.ErrNoAvailImpl)
	assertRoundTrip(t, s, storeSourceFile, func(t *testing.T) {
		path, err := s.filePath()
		require.NoError(t, err)
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(credentialsFilePerm), info.Mode().Perm(),
			"credentials file permission")
		dirInfo, err := os.Stat(filepath.Dir(path))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(credentialsDirPerm), dirInfo.Mode().Perm(),
			"credentials dir permission")
	})
}

func TestDefaultStoreNotFound(t *testing.T) {
	t.Parallel()
	kr := NewMockKeyring(gomock.NewController(t))
	gomock.InOrder(
		kr.EXPECT().Get(storeItem).Return(keyring.Item{}, keyring.ErrKeyNotFound).Times(1),
		kr.EXPECT().Get(storeItem).Return(keyring.Item{}, keyring.ErrKeyNotFound).Times(1),
		kr.EXPECT().Remove(storeItem).Return(nil).Times(1),
	)
	s := testStore(t, kr, nil)
	_, _, found, err := s.Load()
	require.NoError(t, err)
	assert.False(t, found, "found = true, want false when nothing is stored")
	foundAfterDelete, err := s.Delete()
	require.NoError(t, err)
	assert.False(t, foundAfterDelete, "Delete should report not found when nothing is stored")
}

func TestDefaultStoreBrokenCredentials(t *testing.T) {
	t.Parallel()
	s := testStore(t, nil, keyring.ErrNoAvailImpl)
	path, err := s.filePath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), credentialsDirPerm))
	require.NoError(t, os.WriteFile(path, []byte(`{"accessToken":""}`), credentialsFilePerm))
	_, _, _, err = s.Load()
	assert.Error(t, err, "Load should fail on broken credentials")
}
