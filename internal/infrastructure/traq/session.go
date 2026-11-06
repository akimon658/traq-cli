package traq

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/akimon658/traq-cli/internal/domain/auth"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// ResolvedToken は解決済みトークンとその出所 (flag / environment / keychain / file) を保持する。
type ResolvedToken struct {
	Token  *auth.Token
	Source string
}

// ResolveToken はトークンを --token > TRAQ_ACCESS_TOKEN > 保存済みの順で解決する。
// 保存済みトークンが期限切れの場合は refresh を試み、成功すれば保存して返す。
func (s *Service) ResolveToken(
	ctx context.Context,
	tokenOverride, accessToken string,
) (ResolvedToken, error) {
	t := cmp.Or(tokenOverride, accessToken)
	if t != "" {
		source := "environment"
		if tokenOverride != "" {
			source = "flag"
		}
		return ResolvedToken{
			Token:  &auth.Token{Origin: s.origin, AccessToken: t},
			Source: source,
		}, nil
	}
	stored, source, found, err := s.store.Load()
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
		nt, rerr := s.Refresh(ctx, stored)
		if rerr != nil {
			if _, derr := s.store.Delete(); derr != nil {
				fmt.Fprintln(os.Stderr, "warning: failed to delete stored credentials:", derr)
			}
			return ResolvedToken{}, exit.AuthRequiredError(
				fmt.Errorf("refresh failed: %v; run 'traq auth login'", rerr),
			)
		}
		if _, serr := s.store.Save(nt); serr != nil {
			return ResolvedToken{}, serr
		}
		stored = nt
	}
	return ResolvedToken{Token: &stored, Source: source}, nil
}

// RefreshToken は OAuth トークンを更新する。Bot トークン等は更新できない。
func (s *Service) RefreshToken(ctx context.Context, t *auth.Token) (auth.Token, error) {
	if t.Method != auth.MethodOAuth {
		return auth.Token{}, errors.New("this token type cannot be refreshed")
	}
	if t.RefreshToken == "" {
		return auth.Token{}, errors.New("no refresh token available")
	}
	return s.Refresh(ctx, *t)
}
