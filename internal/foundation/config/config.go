package config

import (
	"cmp"
	"os"

	"github.com/akimon658/traq-cli/internal/auth"
)

// Env は実行設定を保持する。
type Env struct {
	Origin   string
	ClientID string
	Store    auth.Store
	// Sessions は手動ログイン (--manual) のセッション保存先。nil の場合は
	// auth.NewDefaultSessionStore を使う。
	Sessions auth.SessionStore
	// AccessToken は TRAQ_ACCESS_TOKEN の値。token のソースは environment として扱う。
	AccessToken string
	// TokenOverride は --token フラグの値。指定時は AccessToken より優先される。
	TokenOverride string
}

// NewEnv は環境変数から Env を作る。
func NewEnv() *Env {
	return &Env{
		Origin:      cmp.Or(os.Getenv("TRAQ_ORIGIN"), auth.DefaultOrigin),
		ClientID:    cmp.Or(os.Getenv("TRAQ_CLIENT_ID"), auth.DefaultClientID),
		Store:       auth.NewDefaultStore(),
		AccessToken: os.Getenv("TRAQ_ACCESS_TOKEN"),
	}
}

// LoadStore は保存先を返す。nil の場合は既定の Store を使う。
func (e *Env) LoadStore() auth.Store {
	if e.Store == nil {
		return auth.NewDefaultStore()
	}
	return e.Store
}

// LoadSessions は手動ログインのセッション保存先を返す。nil の場合は既定を使う。
func (e *Env) LoadSessions() auth.SessionStore {
	if e.Sessions == nil {
		return auth.NewDefaultSessionStore()
	}
	return e.Sessions
}
