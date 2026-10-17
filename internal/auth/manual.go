package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ManualRedirectURI は手動ログイン (traq auth login --manual) で使う redirect URI。
// ポートは登録不要なため登録時と同じ文字列を使う。ローカルサーバーは立てない。
const ManualRedirectURI = "http://127.0.0.1/callback"

// ManualSessionTTL は手動ログインセッションの寿命。
// 認可コードの寿命 (5 分) より長くし、ユーザーがブラウザを開くまでの待ち時間を確保する。
// テストから短縮できるよう変数にしている。
var ManualSessionTTL = 15 * time.Minute

const manualSessionFileName = "login-session.json"

// ManualSession は traq auth login --manual と traq auth complete の間で保持する
// OAuth2 ログインセッション。PKCE の verifier はトークン取得に必要な機密情報であり、
// 保存先は credentials と同じ 0600 にする。
type ManualSession struct {
	Verifier    string    `json:"verifier"`
	State       string    `json:"state"`
	RedirectURI string    `json:"redirectURI"`
	Origin      string    `json:"origin"`
	ClientID    string    `json:"clientId"`
	Scopes      []string  `json:"scopes,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Expired はセッションが有効期限内かを返す。
func (s ManualSession) Expired(now time.Time) bool {
	return now.Sub(s.CreatedAt) > ManualSessionTTL
}

//go:generate go tool mockgen -destination=mock_session.go -package=auth github.com/akimon658/traq-cli/internal/auth SessionStore

type SessionStore interface {
	Save(s ManualSession) (string, error)
	Load(now time.Time) (ManualSession, bool, error)
	Delete() (bool, error)
}

func NewDefaultSessionStore() SessionStore {
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

func (s defaultSessionStore) Save(sess ManualSession) (string, error) {
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
	if err := os.MkdirAll(filepath.Dir(path), credentialsDirPerm); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, credentialsFilePerm); err != nil {
		return "", err
	}
	return path, nil
}

func (s defaultSessionStore) Load(now time.Time) (ManualSession, bool, error) {
	var zero ManualSession
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
	var sess ManualSession
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
