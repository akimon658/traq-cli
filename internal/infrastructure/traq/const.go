package traq

import (
	"time"

	"github.com/akimon658/traq-cli/internal/domain/auth"
)

const (
	// DefaultOrigin は既定の traQ サーバー origin。正典は domain/auth。
	DefaultOrigin = auth.DefaultOrigin
	// DefaultClientID は組み込みの OAuth2 公開クライアント ID。正典は domain/auth。
	DefaultClientID = auth.DefaultClientID

	tokenPath  = "/api/v3/oauth2/token"
	revokePath = "/api/v3/oauth2/revoke"
)

// LoginTimeout はブラウザログイン全体のタイムアウト。テストで短縮できるよう変数にしている。
var LoginTimeout = 5 * time.Minute
