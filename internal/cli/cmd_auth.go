package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
)

func newAuthCmd(newEnv func() *Env, flags *globalFlags) *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "認証サブコマンド (login / logout / status)",
	}
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "traQ にログインする",
		Long: `traQ にログインします。

--browser を付けると OAuth2 authorization code flow (PKCE) でブラウザ経由でトークンを取得します。
  --scope read       読み取り専用 (デフォルト)
  --scope read,write 読み書き (ユーザーと同等の権限)
--token <TOKEN> を付けると、Bot 管理画面などで発行したトークンを直接登録します。
(bot トークンと OAuth トークンのどちらも受け付け、種別は /users/me の応答から自動判別します)

OAuth2 クライアント ID が未登録の場合は TRAQ_CLIENT_ID または --client-id を設定してください。`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogin(cmd, newEnv, flags)
		},
	}
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "traQ からログアウトする",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e := newEnv()
			flags.apply(cmd, e)
			return runLogout(cmd, e)
		},
	}
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "現在の認証状態を表示する",
		RunE: func(cmd *cobra.Command, _ []string) error {
			e := newEnv()
			flags.apply(cmd, e)
			return runStatus(cmd, e)
		},
	}
	loginCmd.Flags().Bool("browser", false, "OAuth2 authorization code flow (PKCE) でトークンを取得する")
	loginCmd.Flags().String("scope", "read", "OAuth2 スコープ (read または read,write)")
	authCmd.AddCommand(loginCmd, logoutCmd, statusCmd)
	return authCmd
}

func runLogin(cmd *cobra.Command, newEnv func() *Env, flags *globalFlags) error {
	e := newEnv()
	flags.apply(cmd, e)
	browser, _ := cmd.Flags().GetBool("browser")
	scopeFlag, _ := cmd.Flags().GetString("scope")

	if cmd.Flags().Changed("token") {
		return loginWithToken(cmd.Context(), cmd.OutOrStdout(), e, e.TokenOverride)
	}
	if !browser {
		return &ExitCodeError{exitUsage, errors.New("specify either --browser or --token <TOKEN>")}
	}
	return login(cmd.Context(), e, scopeFlag, openBrowser, cmd.OutOrStdout())
}

func loginWithToken(ctx context.Context, out io.Writer, e *Env, value string) error {
	c := client.New(e.Origin)
	me, resp, err := c.GetMe(ctx, value)
	if err != nil {
		apiErr := client.WrapError(err, resp)
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return &ExitCodeError{
				exitGeneral,
				fmt.Errorf("the token was rejected: %s", apiErr.Error()),
			}
		}
		return &ExitCodeError{exitGeneral, apiErr}
	}
	t := auth.Token{
		AccessToken: value,
		Origin:      e.Origin,
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
	venue, err := e.store().Save(t)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		out,
		"signed in: @%s (%s)\nstorage: %s\n",
		t.UserName,
		methodLabel(t.Method, t.UserBot),
		venue,
	); err != nil {
		return err
	}
	return nil
}

// loginTimeout はブラウザログイン全体のタイムアウト。テストから短縮できるよう変数にしている。
var loginTimeout = auth.LoginTimeout

func login(
	ctx context.Context,
	e *Env,
	scopeFlag string,
	openBrowser func(url string),
	out io.Writer,
) error {
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	scopes, err := auth.NormalizeScopes(auth.ParseScopes(scopeFlag))
	if err != nil {
		return &ExitCodeError{exitUsage, err}
	}
	pk, err := auth.NewPKCE()
	if err != nil {
		return err
	}
	cb, err := auth.ListenCallback(pk.State)
	if err != nil {
		return err
	}
	defer cb.Close()
	redirectURI := cb.RedirectURI()
	authorizeURL := auth.AuthorizeURL(e.Origin, e.ClientID, redirectURI, scopes, pk)
	if _, err := fmt.Fprintln(out, "Open this URL to authorize traq-cli:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, authorizeURL); err != nil {
		return err
	}
	openBrowser(authorizeURL)
	code, err := cb.Wait(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			err = errors.New("timed out waiting for authorization")
		}
		return err
	}

	resp, err := auth.ExchangeCode(ctx, e.Origin, e.ClientID, redirectURI, code, pk.Verifier)
	if err != nil {
		return err
	}
	scopeList := auth.ParseScopes(resp.Scope)
	if len(scopeList) == 0 {
		scopeList = scopes
	}

	c := client.New(e.Origin)
	me, meResp, err := c.GetMe(ctx, resp.AccessToken)
	if err != nil {
		return &ExitCodeError{exitGeneral, client.WrapError(err, meResp)}
	}
	t := auth.Token{
		Method:       auth.MethodOAuth,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		Scope:        scopeList,
		Origin:       e.Origin,
		UserID:       me.Id,
		UserName:     me.Name,
		UserBot:      me.Bot,
		IssuedAt:     time.Now(),
	}
	if resp.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	}
	venue, err := e.store().Save(t)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		out,
		"signed in: @%s (%s) scopes: %s\nstorage: %s\n",
		t.UserName,
		methodLabel(t.Method, t.UserBot),
		strings.Join(t.Scope, ","),
		venue,
	); err != nil {
		return err
	}
	return nil
}

func runLogout(cmd *cobra.Command, e *Env) error {
	stored, _, found, err := e.store().Load()
	if err != nil {
		return err
	}
	if !found {
		return &ExitCodeError{
			exitAuthRequired,
			errors.New("not authenticated: run 'traq auth login' first"),
		}
	}
	origin := e.OriginOf(&stored)
	if stored.Method == auth.MethodOAuth {
		if rerr := auth.Revoke(cmd.Context(), origin, stored.AccessToken); rerr != nil {
			_, _ = fmt.Fprintln(
				cmd.ErrOrStderr(),
				"failed to revoke the token on the server:",
				rerr,
			)
		}
	}
	if _, err := e.store().Delete(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "logged out"); err != nil {
		return err
	}
	return nil
}

func runStatus(cmd *cobra.Command, e *Env) error {
	rt, err := e.resolveToken(cmd.Context())
	if err != nil {
		return err
	}
	out := map[string]any{
		"origin": e.Origin,
		"source": rt.Source,
	}
	var meErr error
	c := client.New(e.Origin)
	me, meResp, err := c.GetMe(cmd.Context(), rt.Token.AccessToken)
	if err != nil {
		meErr = client.WrapError(err, meResp)
	} else {
		rt.Token.UserID = me.Id
		rt.Token.UserName = me.Name
		rt.Token.UserBot = me.Bot
	}
	var t = rt.Token
	switch t.Method {
	case auth.MethodBot:
		out["method"] = "bot"
	case auth.MethodOAuth:
		out["method"] = "oauth"
		if len(t.Scope) > 0 {
			out["scopes"] = t.Scope
		}
	}
	if t.UserID != "" {
		out["userId"] = t.UserID
	}
	if t.UserName != "" {
		out["user"] = "@" + t.UserName
	}
	if t.UserBot {
		out["method"] = "bot"
	}
	if !t.ExpiresAt.IsZero() {
		out["expiresAt"] = t.ExpiresAt.Format(time.RFC3339)
	}
	if err := printJSON(cmd.OutOrStdout(), out); err != nil {
		return err
	}
	if meErr != nil {
		if meResp != nil && meResp.StatusCode == http.StatusUnauthorized {
			meErr = fmt.Errorf("the token was rejected: %v; run 'traq auth login'", meErr)
		}
		return &ExitCodeError{exitGeneral, fmt.Errorf("failed to verify the token: %v", meErr)}
	}
	return nil
}

func methodLabel(m auth.Method, bot bool) string {
	if bot {
		return "bot"
	}
	if m == auth.MethodOAuth {
		return "user"
	}
	return "unknown"
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}
