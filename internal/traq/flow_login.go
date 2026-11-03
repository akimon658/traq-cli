package traq

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// LoginWithToken は Bot 管理画面等で発行したトークンを検証・保存する。
// bot と OAuth のどちらも受け付け、種別は /users/me の応答から自動判別する。
// buildTokenFromMe に表示用の種別判別を集約している。
func (s *Service) LoginWithToken(
	ctx context.Context,
	value string,
) (Token, string, error) {
	me, resp, err := s.api.MeAPI.GetMe(TokenContext(ctx, value)).Execute()
	if err != nil {
		apiErr := WrapError(err, resp)
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return Token{}, "", exit.GeneralError(
				fmt.Errorf("the token was rejected: %s", apiErr.Error()),
			)
		}
		return Token{}, "", exit.GeneralError(apiErr)
	}
	t := buildTokenFromMe(s.origin, value, me)
	venue, err := s.store.Save(t)
	if err != nil {
		return Token{}, "", err
	}
	return t, venue, nil
}

// BrowserSession はブラウザログインの準備結果を保持する。
type BrowserSession struct {
	PKCE         PKCE
	Callback     *auth.CallbackServer
	RedirectURI  string
	AuthorizeURL string
	Scopes       []string
}

// PrepareBrowserLogin は PKCE 生成・コールバックサーバー起動・認可 URL 組み立てを行う。
// 呼び出し元は AuthorizeURL を表示し、Callback.Wait でコードを待つこと。
func (s *Service) PrepareBrowserLogin(scopeFlag string) (*BrowserSession, error) {
	scopes, err := NormalizeScopes(ParseScopes(scopeFlag))
	if err != nil {
		return nil, exit.UsageError(err)
	}
	pk, err := NewPKCE()
	if err != nil {
		return nil, err
	}
	cb, err := auth.ListenCallback(pk.State)
	if err != nil {
		return nil, err
	}
	redirectURI := cb.RedirectURI()
	return &BrowserSession{
		PKCE:         pk,
		Callback:     cb,
		RedirectURI:  redirectURI,
		AuthorizeURL: pk.AuthorizeURL(s.origin, s.clientID, redirectURI, scopes),
		Scopes:       scopes,
	}, nil
}

// FinishLogin は認可コードをトークンに交換し、/users/me で検証して保存する。
// ブラウザフローと手動フロー (CompleteManualLogin) の共通後半処理。
func (s *Service) FinishLogin(
	ctx context.Context,
	req FinishRequest,
) (Token, string, error) {
	resp, err := s.exchangeCode(ctx, req.Code, req.RedirectURI, req.Verifier)
	if err != nil {
		return Token{}, "", err
	}
	scopeList := ParseScopes(resp.Scope)
	if len(scopeList) == 0 {
		scopeList = req.FallbackScopes
	}

	me, meResp, err := s.api.MeAPI.GetMe(TokenContext(ctx, resp.AccessToken)).Execute()
	if err != nil {
		return Token{}, "", exit.GeneralError(WrapError(err, meResp))
	}
	t := buildTokenFromMe(s.origin, resp.AccessToken, me)
	t.RefreshToken = resp.RefreshToken
	t.Scope = scopeList
	t.IssuedAt = time.Now()
	if resp.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	}
	venue, err := s.store.Save(t)
	if err != nil {
		return Token{}, "", err
	}
	return t, venue, nil
}
