package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// StartManualLogin は --manual のセッションを保存し、認可 URL を返す。
func StartManualLogin(
	origin, clientID, scopeFlag string,
	sessions SessionStore,
) (string, error) {
	if clientID == "" {
		return "", exit.UsageError(
			errors.New(
				"OAuth client ID is not configured; set TRAQ_CLIENT_ID or --client-id (see docs/auth.md)",
			),
		)
	}
	scopes, err := NormalizeScopes(ParseScopes(scopeFlag))
	if err != nil {
		return "", exit.UsageError(err)
	}
	pk, err := NewPKCE()
	if err != nil {
		return "", err
	}
	sess := ManualSession{
		Verifier:    pk.Verifier,
		State:       pk.State,
		RedirectURI: ManualRedirectURI,
		Origin:      origin,
		ClientID:    clientID,
		Scopes:      scopes,
		CreatedAt:   time.Now(),
	}
	if _, err := sessions.Save(sess); err != nil {
		return "", err
	}
	return AuthorizeURL(sess.Origin, sess.ClientID, sess.RedirectURI, scopes, pk), nil
}

// ParseCallback は traq auth complete の引数から code と state を取り出す。
// 完全な URL のほか、"?code=..." や "code=...&state=..." のようなクエリ文字列も受け付ける。
// 形式が不正なら err を (usage エラー)、ユーザーが拒否したなら denied を返す。
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

// CompleteManualLogin は手動ログインを完了し、トークンと保存先を返す。
func CompleteManualLogin(
	ctx context.Context,
	origin, clientID string,
	c *client.Client,
	store Store,
	sessions SessionStore,
	callback string,
) (Token, string, error) {
	code, state, denied, err := ParseCallback(callback)
	if err != nil {
		return Token{}, "", exit.UsageError(err)
	}
	if denied != nil {
		return Token{}, "", exit.GeneralError(denied)
	}
	sess, found, err := sessions.Load(time.Now())
	if err != nil {
		return Token{}, "", err
	}
	if !found {
		return Token{}, "", exit.AuthRequiredError(
			errors.New("no pending manual login: run 'traq auth login --manual' first"),
		)
	}
	if state != "" && state != sess.State {
		return Token{}, "", exit.GeneralError(errors.New(
			"authorization callback state mismatch; " +
				"re-run 'traq auth login --manual' and use the new URL",
		))
	}

	sessOrigin := origin
	if sess.Origin != "" {
		sessOrigin = sess.Origin
	}
	sessClientID := clientID
	if sess.ClientID != "" {
		sessClientID = sess.ClientID
	}
	t, venue, err := FinishLogin(
		ctx,
		sessOrigin,
		sessClientID,
		c,
		store,
		code,
		sess.RedirectURI,
		sess.Verifier,
		sess.Scopes,
	)
	if err != nil {
		return Token{}, "", err
	}
	// 認可コードは 1 回限りなので、トークン保存まで成功したらセッションを消す
	if _, err := sessions.Delete(); err != nil {
		return Token{}, "", err
	}
	return t, venue, nil
}
