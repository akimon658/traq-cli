package traq

import (
	"time"
)

const (
	tokenPath  = "/api/v3/oauth2/token"
	revokePath = "/api/v3/oauth2/revoke"
)

// LoginTimeout はブラウザログイン全体のタイムアウト。テストで短縮できるよう変数にしている。
var LoginTimeout = 5 * time.Minute
