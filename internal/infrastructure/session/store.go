package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/akimon658/traq-cli/internal/domain/auth"
)

const manualSessionFileName = "login-session.json"

// セッション保存先のパーミッション。credentials と同じく 0600/0700 にする。
const (
	sessionDirPerm  = 0o700
	sessionFilePerm = 0o600
)

func NewDefaultSessionStore() auth.SessionStore {
	return defaultSessionStore{userConfigDir: os.UserConfigDir}
}

type defaultSessionStore struct {
	userConfigDir func() (string, error)
}

func (s defaultSessionStore) filePath() (string, error) {
	dir, err := s.userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "traq-cli", manualSessionFileName), nil
}

func (s defaultSessionStore) Save(sess auth.ManualSession) (string, error) {
	if sess.Verifier == "" || sess.State == "" {
		return "", errors.New("manual login session is broken: no PKCE verifier or state")
	}
	data, err := json.Marshal(sess)
	if err != nil {
		return "", err
	}
	path, err := s.filePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), sessionDirPerm); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, sessionFilePerm); err != nil {
		return "", err
	}
	return path, nil
}

func (s defaultSessionStore) Load(now time.Time) (auth.ManualSession, bool, error) {
	var zero auth.ManualSession
	path, err := s.filePath()
	if err != nil {
		return zero, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return zero, false, nil
		}
		return zero, false, err
	}
	var sess auth.ManualSession
	if err := json.Unmarshal(data, &sess); err != nil {
		if derr := os.Remove(path); derr != nil && !errors.Is(derr, os.ErrNotExist) {
			return zero, false, fmt.Errorf(
				"failed to remove a broken manual login session: %w",
				derr,
			)
		}
		return zero, false, errors.New(
			"stored manual login session is broken; run 'traq auth login --manual' again",
		)
	}
	if sess.Verifier == "" || sess.State == "" {
		if derr := os.Remove(path); derr != nil && !errors.Is(derr, os.ErrNotExist) {
			return zero, false, fmt.Errorf(
				"failed to remove a broken manual login session: %w",
				derr,
			)
		}
		return zero, false, errors.New(
			"stored manual login session is broken; run 'traq auth login --manual' again",
		)
	}
	if sess.Expired(now) {
		if _, derr := s.Delete(); derr != nil {
			return zero, false, derr
		}
		return zero, false, nil
	}
	return sess, true, nil
}

func (s defaultSessionStore) Delete() (bool, error) {
	path, err := s.filePath()
	if err != nil {
		return false, err
	}
	found := false
	if _, serr := os.Stat(path); serr == nil {
		found = true
	}
	if rerr := os.Remove(path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return found, rerr
	}
	return found, nil
}
