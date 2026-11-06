package config

import (
	"cmp"
	"os"

	"github.com/akimon658/traq-cli/internal/domain/auth"
)

// Env は実行設定を保持する。
type Env struct {
	Origin   string
	ClientID string
	Store    auth.Store
	// Sessions は手動ログイン (--manual) のセッション保存先。nil の場合は
	// infrastructure/session の既定実装を使う。
	Sessions auth.SessionStore
	// AccessToken は TRAQ_ACCESS_TOKEN の値。token のソースは environment として扱う。
	AccessToken string
	// TokenOverride は --token フラグの値。指定時は AccessToken より優先される。
	TokenOverride string
}

// NewEnv は環境変数から Env を作る。
// Store/Sessions の具象生成は行わない。必要なら ui/cli の配線で注入すること。
func NewEnv() *Env {
	return &Env{
		Origin:      cmp.Or(os.Getenv("TRAQ_ORIGIN"), auth.DefaultOrigin),
		ClientID:    cmp.Or(os.Getenv("TRAQ_CLIENT_ID"), auth.DefaultClientID),
		AccessToken: os.Getenv("TRAQ_ACCESS_TOKEN"),
	}
}
