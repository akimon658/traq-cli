package traq

import (
	"context"
	"errors"
	"time"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// StartManualLogin は --manual のセッションを保存し、認可 URL を返す。
func (s *Service) StartManualLogin(scopeFlag string) (string, error) {
	if s.clientID == "" {
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
	sess := auth.ManualSession{
		Verifier:    pk.Verifier,
		State:       pk.State,
		RedirectURI: auth.ManualRedirectURI,
		Origin:      s.origin,
		ClientID:    s.clientID,
		Scopes:      scopes,
		CreatedAt:   time.Now(),
	}
	if _, err := s.sessions.Save(sess); err != nil {
		return "", err
	}
	return pk.AuthorizeURL(sess.Origin, sess.ClientID, sess.RedirectURI, scopes), nil
}

// CompleteManualLogin は手動ログインを完了し、トークンと保存先を返す。
func (s *Service) CompleteManualLogin(
	ctx context.Context,
	callback string,
) (Token, string, error) {
	code, state, denied, err := auth.ParseCallback(callback)
	if err != nil {
		return Token{}, "", exit.UsageError(err)
	}
	if denied != nil {
		return Token{}, "", exit.GeneralError(denied)
	}
	sess, found, err := s.sessions.Load(time.Now())
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

	sessOrigin := s.origin
	if sess.Origin != "" {
		sessOrigin = sess.Origin
	}
	sessClientID := s.clientID
	if sess.ClientID != "" {
		sessClientID = sess.ClientID
	}
	// セッション保存時の origin/clientID を優先する。ここでの NewService は
	// auth 内部で完結する唯一の例外生成点であり、呼び出し側で client を重ねて作らない。
	finishSvc := NewService(sessOrigin, sessClientID, s.store, s.sessions)
	t, venue, err := finishSvc.FinishLogin(
		ctx,
		FinishRequest{
			Code:           code,
			RedirectURI:    sess.RedirectURI,
			Verifier:       sess.Verifier,
			FallbackScopes: sess.Scopes,
		},
	)
	if err != nil {
		return Token{}, "", err
	}
	// 認可コードは 1 回限りなので、トークン保存まで成功したらセッションを消す
	if _, err := s.sessions.Delete(); err != nil {
		return Token{}, "", err
	}
	return t, venue, nil
}
