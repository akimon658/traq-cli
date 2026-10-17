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

func StorePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "traq-cli", "credentials"), nil
}

func Save(t Token) (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	if venue := saveKeyring(data); venue != "" {
		if path, ferr := StorePath(); ferr == nil {
			_ = os.Remove(path)
		}
		return venue, nil
	}
	path, err := StorePath()
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

func Load() (Token, string, bool, error) {
	var zero Token
	if r, err := keyring.Open(
		keyring.Config{ServiceName: storeService, AllowedBackends: storeBackends},
	); err == nil {
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
	path, err := StorePath()
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

func Delete() (bool, error) {
	found := false
	if r, err := keyring.Open(
		keyring.Config{ServiceName: storeService, AllowedBackends: storeBackends},
	); err == nil {
		if _, e := r.Get(storeItem); e == nil {
			found = true
		}
		_ = r.Remove(storeItem)
	}
	path, err := StorePath()
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

func saveKeyring(data []byte) string {
	r, err := keyring.Open(keyring.Config{
		ServiceName:     storeService,
		AllowedBackends: storeBackends,
	})
	if err != nil {
		return ""
	}
	if err := r.Set(keyring.Item{Key: storeItem, Data: data}); err != nil {
		return ""
	}
	return storeSourceKeychain
}
