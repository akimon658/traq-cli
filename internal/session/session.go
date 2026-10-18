package session

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
)

// ResolvedToken は解決済みトークンとその出所 (flag / environment / keychain / file) を保持する。
type ResolvedToken struct {
	Token  *auth.Token
	Source string
}

// ResolveToken はトークンを --token > TRAQ_ACCESS_TOKEN > 保存済みの順で解決する。
// 保存済みトークンが期限切れの場合は refresh を試み、成功すれば保存して返す。
func ResolveToken(ctx context.Context, e *config.Env) (ResolvedToken, error) {
	t := cmp.Or(e.TokenOverride, e.AccessToken)
	if t != "" {
		source := "environment"
		if e.TokenOverride != "" {
			source = "flag"
		}
		return ResolvedToken{
			Token:  &auth.Token{Origin: e.Origin, AccessToken: t},
			Source: source,
		}, nil
	}
	stored, source, found, err := e.LoadStore().Load()
	if err != nil {
		return ResolvedToken{}, err
	}
	if !found {
		return ResolvedToken{}, exit.AuthRequiredError(
			errors.New("not authenticated: run 'traq auth login'"),
		)
	}
	if stored.Expired() {
		if stored.RefreshToken == "" {
			return ResolvedToken{}, exit.AuthRequiredError(
				errors.New("the stored token has expired: run 'traq auth login'"),
			)
		}
		nt, rerr := auth.Refresh(ctx, e.OriginOf(&stored), e.ClientID, stored)
		if rerr != nil {
			_, _ = e.LoadStore().Delete()
			return ResolvedToken{}, exit.AuthRequiredError(
				fmt.Errorf("refresh failed: %v; run 'traq auth login'", rerr),
			)
		}
		if _, serr := e.LoadStore().Save(nt); serr != nil {
			return ResolvedToken{}, serr
		}
		stored = nt
	}
	return ResolvedToken{Token: &stored, Source: source}, nil
}

// Exec は API を実行し、401 なら refresh して1回だけリトライする。
// refresh 後のトークンは保存先に書き戻し、呼び出し元の ResolvedToken も更新する。
func Exec(
	ctx context.Context,
	e *config.Env,
	rt *ResolvedToken,
	fn func(ctx context.Context) (*http.Response, error),
) error {
	resp, err := fn(client.TokenContext(ctx, rt.Token.AccessToken))
	if resp != nil && resp.StatusCode == http.StatusUnauthorized {
		nt, rerr := RefreshToken(ctx, e, rt.Token)
		if rerr != nil {
			return exit.AuthRequiredError(
				fmt.Errorf(
					"the token was rejected and cannot be refreshed (%v); run 'traq auth login'",
					rerr,
				),
			)
		}
		if _, serr := e.LoadStore().Save(nt); serr != nil {
			return serr
		}
		*rt.Token = nt
		resp, err = fn(client.TokenContext(ctx, rt.Token.AccessToken))
	}
	if err != nil {
		return exit.GeneralError(client.WrapError(err, resp))
	}
	return nil
}

// RefreshToken は OAuth トークンを更新する。Bot トークン等は更新できない。
func RefreshToken(ctx context.Context, e *config.Env, t *auth.Token) (auth.Token, error) {
	if t.Method != auth.MethodOAuth {
		return auth.Token{}, errors.New("this token type cannot be refreshed")
	}
	if t.RefreshToken == "" {
		return auth.Token{}, errors.New("no refresh token available")
	}
	return auth.Refresh(ctx, e.OriginOf(t), e.ClientID, *t)
}

// RequireScope は保存済み OAuth トークンのスコープを事前検査する。
// Bot トークンやスコープ不明な手動トークン (--token) は検査せず、サーバーの判定に委ねる。
func RequireScope(rt *ResolvedToken, scope string) error {
	if rt.Token.LacksScope(scope) {
		required := scope
		if scope == auth.ScopeWrite {
			required = auth.ScopeRead + "," + auth.ScopeWrite
		}
		return exit.ScopeInsufficientError(
			fmt.Errorf(
				"the current token does not have the '%s' scope; re-run 'traq auth login --browser --scope %s'",
				scope,
				required,
			),
		)
	}
	return nil
}
