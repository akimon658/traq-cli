package authflow

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
)

// StartManualLogin は --manual のセッションを保存し、認可 URL を返す。
func StartManualLogin(e *config.Env, scopeFlag string) (string, error) {
	if e.ClientID == "" {
		return "", exit.UsageError(
			errors.New(
				"OAuth client ID is not configured; set TRAQ_CLIENT_ID or --client-id (see docs/auth.md)",
			),
		)
	}
	scopes, err := auth.NormalizeScopes(auth.ParseScopes(scopeFlag))
	if err != nil {
		return "", exit.UsageError(err)
	}
	pk, err := auth.NewPKCE()
	if err != nil {
		return "", err
	}
	sess := auth.ManualSession{
		Verifier:    pk.Verifier,
		State:       pk.State,
		RedirectURI: auth.ManualRedirectURI,
		Origin:      e.Origin,
		ClientID:    e.ClientID,
		Scopes:      scopes,
		CreatedAt:   time.Now(),
	}
	if _, err := e.LoadSessions().Save(sess); err != nil {
		return "", err
	}
	return auth.AuthorizeURL(sess.Origin, sess.ClientID, sess.RedirectURI, scopes, pk), nil
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
	e *config.Env,
	c *client.Client,
	callback string,
) (auth.Token, string, error) {
	code, state, denied, err := ParseCallback(callback)
	if err != nil {
		return auth.Token{}, "", exit.UsageError(err)
	}
	if denied != nil {
		return auth.Token{}, "", exit.GeneralError(denied)
	}
	sess, found, err := e.LoadSessions().Load(time.Now())
	if err != nil {
		return auth.Token{}, "", err
	}
	if !found {
		return auth.Token{}, "", exit.AuthRequiredError(
			errors.New("no pending manual login: run 'traq auth login --manual' first"),
		)
	}
	if state != "" && state != sess.State {
		return auth.Token{}, "", exit.GeneralError(errors.New(
			"authorization callback state mismatch; " +
				"re-run 'traq auth login --manual' and use the new URL",
		))
	}

	sessEnv := *e
	sessEnv.Origin = sess.Origin
	sessEnv.ClientID = sess.ClientID
	t, venue, err := FinishLogin(
		ctx,
		&sessEnv,
		c,
		code,
		sess.RedirectURI,
		sess.Verifier,
		sess.Scopes,
	)
	if err != nil {
		return auth.Token{}, "", err
	}
	// 認可コードは 1 回限りなので、トークン保存まで成功したらセッションを消す
	if _, err := e.LoadSessions().Delete(); err != nil {
		return auth.Token{}, "", err
	}
	return t, venue, nil
}
