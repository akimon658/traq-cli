package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
)

type Env struct {
	Origin   string
	ClientID string
	Store    auth.Store
}

func NewEnv() *Env {
	return &Env{
		Origin:   firstNonEmpty(flagOrigin, os.Getenv("TRAQ_ORIGIN"), auth.DefaultOrigin),
		ClientID: firstNonEmpty(flagClientID, os.Getenv("TRAQ_CLIENT_ID"), auth.DefaultClientID),
		Store:    auth.NewDefaultStore(),
	}
}

func (e *Env) store() auth.Store {
	if e.Store == nil {
		return auth.NewDefaultStore()
	}
	return e.Store
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

type ResolvedToken struct {
	Token  *auth.Token
	Source string
}

func (e *Env) resolveToken() (ResolvedToken, error) {
	t := firstNonEmpty(flagToken, os.Getenv("TRAQ_ACCESS_TOKEN"))
	if t != "" {
		source := "environment"
		if flagToken != "" {
			source = "flag"
		}
		return ResolvedToken{
			Token:  &auth.Token{Origin: e.Origin, AccessToken: t},
			Source: source,
		}, nil
	}
	stored, source, found, err := e.store().Load()
	if err != nil {
		return ResolvedToken{}, err
	}
	if !found {
		return ResolvedToken{}, &ExitCodeError{
			exitAuthRequired,
			errors.New("not authenticated: run 'traq auth login'"),
		}
	}
	if stored.Expired() {
		if stored.RefreshToken == "" {
			return ResolvedToken{}, &ExitCodeError{
				exitAuthRequired,
				errors.New("the stored token has expired: run 'traq auth login'"),
			}
		}
		nt, rerr := auth.Refresh(context.Background(), e.OriginOf(&stored), e.ClientID, stored)
		if rerr != nil {
			_, _ = e.store().Delete()
			return ResolvedToken{}, &ExitCodeError{
				exitAuthRequired,
				fmt.Errorf("refresh failed: %v; run 'traq auth login'", rerr),
			}
		}
		if _, serr := e.store().Save(nt); serr != nil {
			return ResolvedToken{}, serr
		}
		stored = nt
	}
	return ResolvedToken{Token: &stored, Source: source}, nil
}

func (e *Env) OriginOf(t *auth.Token) string {
	if t.Origin != "" {
		return t.Origin
	}
	return e.Origin
}

func (e *Env) apiExec(
	ctx context.Context,
	rt *ResolvedToken,
	fn func(ctx context.Context) (*http.Response, error),
) error {
	resp, err := fn(client.TokenContext(ctx, rt.Token.AccessToken))
	if resp != nil && resp.StatusCode == http.StatusUnauthorized {
		nt, rerr := e.refreshToken(ctx, rt.Token)
		if rerr != nil {
			return &ExitCodeError{
				exitAuthRequired,
				fmt.Errorf(
					"the token was rejected and cannot be refreshed (%v); run 'traq auth login'",
					rerr,
				),
			}
		}
		if _, serr := e.store().Save(nt); serr != nil {
			return serr
		}
		*rt.Token = nt
		resp, err = fn(client.TokenContext(ctx, rt.Token.AccessToken))
	}
	if err != nil {
		return &ExitCodeError{exitGeneral, client.WrapError(err, resp)}
	}
	return nil
}

func (e *Env) refreshToken(ctx context.Context, t *auth.Token) (auth.Token, error) {
	if t.Method != auth.MethodOAuth {
		return auth.Token{}, errors.New("this token type cannot be refreshed")
	}
	if t.RefreshToken == "" {
		return auth.Token{}, errors.New("no refresh token available")
	}
	return auth.Refresh(ctx, e.OriginOf(t), e.ClientID, *t)
}

func (e *Env) requireScope(rt *ResolvedToken, scope string) error {
	if rt.Token.LacksScope(scope) {
		required := scope
		if scope == auth.ScopeWrite {
			required = auth.ScopeRead + "," + auth.ScopeWrite
		}
		return &ExitCodeError{
			exitScopeInsufficient,
			fmt.Errorf(
				"the current token does not have the '%s' scope; re-run 'traq auth login --browser --scope %s'",
				scope,
				required,
			),
		}
	}
	return nil
}
