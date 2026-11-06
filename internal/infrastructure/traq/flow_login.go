package traq

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gotraq "github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/domain/auth"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// LoginWithToken は Bot 管理画面等で発行したトークンを検証・保存する。
// bot と OAuth のどちらも受け付け、種別は /users/me の応答から自動判別する。
// buildTokenFromMe に表示用の種別判別を集約している。
func (s *Service) LoginWithToken(
	ctx context.Context,
	value string,
) (auth.Token, string, error) {
	me, resp, err := s.api.MeAPI.GetMe(TokenContext(ctx, value)).Execute()
	if err != nil {
		apiErr := WrapError(err, resp)
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return auth.Token{}, "", exit.GeneralError(
				fmt.Errorf("the token was rejected: %s", apiErr.Error()),
			)
		}
		return auth.Token{}, "", exit.GeneralError(apiErr)
	}
	t := buildTokenFromMe(s.origin, value, me)
	venue, err := s.store.Save(t)
	if err != nil {
		return auth.Token{}, "", err
	}
	return t, venue, nil
}

// BrowserSession はブラウザログインの準備結果を保持する。
type BrowserSession struct {
	PKCE         auth.PKCE
	Callback     *CallbackServer
	RedirectURI  string
	AuthorizeURL string
	Scopes       []string
}

// PrepareBrowserLogin は PKCE 生成・コールバックサーバー起動・認可 URL 組み立てを行う。
// 呼び出し元は AuthorizeURL を表示し、Callback.Wait でコードを待つこと。
func (s *Service) PrepareBrowserLogin(scopeFlag string) (*BrowserSession, error) {
	scopes, err := auth.NormalizeScopes(auth.ParseScopes(scopeFlag))
	if err != nil {
		return nil, exit.UsageError(err)
	}
	pk, err := auth.NewPKCE()
	if err != nil {
		return nil, err
	}
	cb, err := ListenCallback(pk.State)
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
) (auth.Token, string, error) {
	resp, err := s.exchangeCode(ctx, req.Code, req.RedirectURI, req.Verifier)
	if err != nil {
		return auth.Token{}, "", err
	}
	scopeList := auth.ParseScopes(resp.Scope)
	if len(scopeList) == 0 {
		scopeList = req.FallbackScopes
	}

	me, meResp, err := s.api.MeAPI.GetMe(TokenContext(ctx, resp.AccessToken)).Execute()
	if err != nil {
		return auth.Token{}, "", exit.GeneralError(WrapError(err, meResp))
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
		return auth.Token{}, "", err
	}
	return t, venue, nil
}

// buildTokenFromMe は /users/me の応答から保存用 Token を組み立てる。
// bot と OAuth の種別判別を一箇所に集約するためのヘルパー。
func buildTokenFromMe(origin, accessToken string, me *gotraq.MyUserDetail) auth.Token {
	t := auth.Token{
		AccessToken: accessToken,
		Origin:      origin,
		UserID:      me.Id,
		UserName:    me.Name,
		UserBot:     me.Bot,
		IssuedAt:    time.Now(),
	}
	if me.Bot {
		t.Method = auth.MethodBot
	} else {
		t.Method = auth.MethodOAuth
	}
	return t
}
