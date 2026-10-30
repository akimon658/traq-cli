package auth

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// ResolvedToken は解決済みトークンとその出所 (flag / environment / keychain / file) を保持する。
type ResolvedToken struct {
	Token  *Token
	Source string
}

// OriginOr はトークンに記録された Origin を優先し、無ければ fallback を返す。
func (t *Token) OriginOr(fallback string) string {
	if t == nil || t.Origin == "" {
		return fallback
	}
	return t.Origin
}

// ResolveToken はトークンを --token > TRAQ_ACCESS_TOKEN > 保存済みの順で解決する。
// 保存済みトークンが期限切れの場合は refresh を試み、成功すれば保存して返す。
func ResolveToken(
	ctx context.Context,
	origin, clientID, tokenOverride, accessToken string,
	store Store,
) (ResolvedToken, error) {
	t := cmp.Or(tokenOverride, accessToken)
	if t != "" {
		source := "environment"
		if tokenOverride != "" {
			source = "flag"
		}
		return ResolvedToken{
			Token:  &Token{Origin: origin, AccessToken: t},
			Source: source,
		}, nil
	}
	stored, source, found, err := store.Load()
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
		nt, rerr := Refresh(ctx, stored.OriginOr(origin), clientID, stored)
		if rerr != nil {
			_, _ = store.Delete()
			return ResolvedToken{}, exit.AuthRequiredError(
				fmt.Errorf("refresh failed: %v; run 'traq auth login'", rerr),
			)
		}
		if _, serr := store.Save(nt); serr != nil {
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
	origin, clientID string,
	store Store,
	rt *ResolvedToken,
	fn func(ctx context.Context) (*http.Response, error),
) error {
	resp, err := fn(client.TokenContext(ctx, rt.Token.AccessToken))
	if resp != nil && resp.StatusCode == http.StatusUnauthorized {
		nt, rerr := RefreshToken(ctx, origin, clientID, rt.Token)
		if rerr != nil {
			return exit.AuthRequiredError(
				fmt.Errorf(
					"the token was rejected and cannot be refreshed (%v); run 'traq auth login'",
					rerr,
				),
			)
		}
		if _, serr := store.Save(nt); serr != nil {
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
func RefreshToken(ctx context.Context, origin, clientID string, t *Token) (Token, error) {
	if t.Method != MethodOAuth {
		return Token{}, errors.New("this token type cannot be refreshed")
	}
	if t.RefreshToken == "" {
		return Token{}, errors.New("no refresh token available")
	}
	return Refresh(ctx, t.OriginOr(origin), clientID, *t)
}

// RequireScope は保存済み OAuth トークンのスコープを事前検査する。
// Bot トークンやスコープ不明な手動トークン (--token) は検査せず、サーバーの判定に委ねる。
func RequireScope(rt *ResolvedToken, scope string) error {
	if rt.Token.LacksScope(scope) {
		required := scope
		if scope == ScopeWrite {
			required = ScopeRead + "," + ScopeWrite
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
