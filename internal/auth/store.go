package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/99designs/keyring"
)

const (
	storeService = "traq-cli"
	storeItem    = "default"

	storeSourceKeychain = "keychain"
	storeSourceFile     = "file"
)

var storeBackends = []keyring.BackendType{
	keyring.KeychainBackend,
	keyring.SecretServiceBackend,
	keyring.KWalletBackend,
	keyring.KeyCtlBackend,
	keyring.WinCredBackend,
}

const (
	credentialsDirPerm  = 0o700
	credentialsFilePerm = 0o600
)

type Store interface {
	Save(t Token) (string, error)
	Load() (Token, string, bool, error)
	Delete() (bool, error)
}

func NewDefaultStore() Store {
	return defaultStore{
		openKeyring:   openDefaultKeyring,
		userConfigDir: os.UserConfigDir,
	}
}

func openDefaultKeyring() (keyring.Keyring, error) {
	return keyring.Open(keyring.Config{
		ServiceName:     storeService,
		AllowedBackends: storeBackends,
	})
}

type defaultStore struct {
	openKeyring   func() (keyring.Keyring, error)
	userConfigDir func() (string, error)
}

func (s defaultStore) filePath() (string, error) {
	dir, err := s.userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "traq-cli", "credentials"), nil
}

func (s defaultStore) Save(t Token) (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	if venue := s.saveKeyring(data); venue != "" {
		if path, ferr := s.filePath(); ferr == nil {
			_ = os.Remove(path)
		}
		return venue, nil
	}
	path, err := s.filePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), credentialsDirPerm); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, credentialsFilePerm); err != nil {
		return "", err
	}
	return storeSourceFile, nil
}

func (s defaultStore) Load() (Token, string, bool, error) {
	var zero Token
	if r, err := s.openKeyring(); err == nil {
		item, e := r.Get(storeItem)
		if e == nil {
			var t Token
			if err := json.Unmarshal(item.Data, &t); err != nil {
				return zero, "", false, err
			}
			if t.AccessToken == "" {
				return zero, "", false, errors.New(
					"stored credentials are broken; run 'traq auth login' again",
				)
			}
			return t, storeSourceKeychain, true, nil
		}
		if !errors.Is(e, keyring.ErrKeyNotFound) && !errors.Is(e, keyring.ErrNoAvailImpl) {
			return zero, "", false, e
		}
	}
	path, err := s.filePath()
	if err != nil {
		return zero, "", false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return zero, "", false, nil
		}
		return zero, "", false, err
	}
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return zero, "", false, err
	}
	if t.AccessToken == "" {
		return zero, "", false, errors.New(
			"stored credentials are broken; run 'traq auth login' again",
		)
	}
	return t, storeSourceFile, true, nil
}

func (s defaultStore) Delete() (bool, error) {
	found := false
	if r, err := s.openKeyring(); err == nil {
		if _, e := r.Get(storeItem); e == nil {
			found = true
		}
		_ = r.Remove(storeItem)
	}
	path, err := s.filePath()
	if err != nil {
		return found, err
	}
	if _, serr := os.Stat(path); serr == nil {
		found = true
	}
	if rerr := os.Remove(path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return found, rerr
	}
	return found, nil
}

func (s defaultStore) saveKeyring(data []byte) string {
	r, err := s.openKeyring()
	if err != nil {
		return ""
	}
	if err := r.Set(keyring.Item{Key: storeItem, Data: data}); err != nil {
		return ""
	}
	return storeSourceKeychain
}
