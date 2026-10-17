package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func ExchangeCode(
	ctx context.Context,
	origin, clientID, redirectURI, code, verifier string,
) (tokenResponse, error) {
	if clientID == "" {
		return tokenResponse{}, errors.New(
			"OAuth client ID is not configured; run `traq auth login --browser` after setting TRAQ_CLIENT_ID or --client-id (see docs/auth.md)",
		)
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	return requestToken(ctx, origin, form)
}

func Refresh(ctx context.Context, origin, clientID string, prev Token) (Token, error) {
	if prev.RefreshToken == "" {
		return Token{}, errors.New("no refresh token available")
	}
	if clientID == "" {
		return Token{}, errors.New(
			"OAuth client ID is not configured; set TRAQ_CLIENT_ID or --client-id (see docs/auth.md)",
		)
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {prev.RefreshToken},
		"client_id":     {clientID},
	}
	resp, err := requestToken(ctx, origin, form)
	if err != nil {
		return Token{}, err
	}
	t := prev
	t.AccessToken = resp.AccessToken
	if resp.RefreshToken != "" {
		t.RefreshToken = resp.RefreshToken
	}
	now := time.Now()
	t.IssuedAt = now
	if resp.ExpiresIn > 0 {
		t.ExpiresAt = now.Add(time.Duration(resp.ExpiresIn) * time.Second)
	} else {
		t.ExpiresAt = time.Time{}
	}
	if resp.Scope != "" {
		t.Scope = ParseScopes(resp.Scope)
	}
	return t, nil
}

const (
	httpStatusMinSuccess = 200
	httpStatusMaxSuccess = 299
)

func Revoke(ctx context.Context, origin, accessToken string) error {
	form := url.Values{"token": {accessToken}}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		origin+revokePath,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < httpStatusMinSuccess || resp.StatusCode > httpStatusMaxSuccess {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to revoke the token (HTTP %d)", resp.StatusCode)
		}
		return fmt.Errorf(
			"failed to revoke the token (HTTP %d): %s",
			resp.StatusCode,
			strings.TrimRight(string(body), "\n"),
		)
	}
	return nil
}

func requestToken(ctx context.Context, origin string, form url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		origin+tokenPath,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("token request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return tokenResponse{}, fmt.Errorf(
			"token request failed (HTTP %d): %s",
			resp.StatusCode,
			strings.TrimRight(string(body), "\n"),
		)
	}
	var res tokenResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return tokenResponse{}, err
	}
	if res.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf(
			"token request failed: no access token in the response: %s",
			strings.TrimRight(string(body), "\n"),
		)
	}
	return res, nil
}
