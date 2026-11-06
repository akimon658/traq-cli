package auth

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ManualRedirectURI は手動ログイン (traq auth login --manual) で使う redirect URI。
// ポートは登録不要なため登録時と同じ文字列を使う。ローカルサーバーは立てない。
const ManualRedirectURI = "http://127.0.0.1/callback"

// ManualSessionTTL は手動ログインセッションの寿命。
// 認可コードの寿命 (5 分) より長くし、ユーザーがブラウザを開くまでの待ち時間を確保する。
// テストから短縮できるよう変数にしている。
var ManualSessionTTL = 15 * time.Minute

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

//go:generate go tool mockgen -destination=mock_session.go -package=auth github.com/akimon658/traq-cli/internal/domain/auth SessionStore

type SessionStore interface {
	Save(s ManualSession) (string, error)
	Load(now time.Time) (ManualSession, bool, error)
	Delete() (bool, error)
}

// ParseCallback は手動ログインのリダイレクト先 URL から code と state を取り出す。
// 完全な URL のほか、"?code=..." や "code=...&state=..." のようなクエリ文字列も受け付ける。
// 形式が不正なら err を、ユーザーが拒否したなら denied を返す。
func ParseCallback(raw string) (code, state string, denied, err error) {
	raw = strings.TrimSpace(raw)
	var q url.Values
	if u, uerr := url.Parse(raw); uerr == nil && u.RawQuery != "" {
		q = u.Query()
	} else {
		trimmed := strings.TrimPrefix(raw, "?")
		q, err = url.ParseQuery(trimmed)
		if err != nil {
			return "", "", nil, errors.New("the callback is neither a URL nor a query string")
		}
	}
	if e := q.Get("error"); e != "" {
		d := q.Get("error_description")
		if d != "" {
			return "", "", fmt.Errorf("authorization was denied (%s: %s)", e, d), nil
		}
		return "", "", fmt.Errorf("authorization was denied (%s)", e), nil
	}
	if q.Get("code") == "" {
		return "", "", nil, errors.New("no code in the callback")
	}
	return q.Get("code"), q.Get("state"), nil, nil
}
