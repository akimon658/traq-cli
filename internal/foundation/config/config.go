package config

import (
	"cmp"
	"os"

	"github.com/akimon658/traq-cli/internal/auth"
)

// Env は実行設定を保持する。値の出所は次の優先順位で解決される。
// 1. フラグ (--token / --origin / --client-id)
// 2. 環境変数 (TRAQ_ACCESS_TOKEN / TRAQ_ORIGIN / TRAQ_CLIENT_ID)
// 3. 保存済みトークン (keyring または credentials ファイル)
//
// Env 自体は値の保持と既定値の解決だけを担う。
// トークンの解決・更新・401リトライは session パッケージが担当する。
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

// OriginOf はトークンに記録された Origin を優先し、無ければ Env の Origin を返す。
func (e *Env) OriginOf(t *auth.Token) string {
	if t.Origin != "" {
		return t.Origin
	}
	return e.Origin
}
