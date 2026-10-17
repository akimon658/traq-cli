package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
--manual を付けるとブラウザを開かずに認可 URL のみを出力します。
ブラウザで承認した後にリダイレクトされる URL (開けなくても問題ありません) を
traq auth complete に渡すことでログインを完了します。
SSH 先や headless 環境など、ブラウザが同マシンで開けない場合に使います。
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
	completeCmd := &cobra.Command{
		Use:   "complete <URL>",
		Short: "手動ログイン (login --manual) を完了する",
		Long: `traq auth login --manual で発行した認可 URL でブラウザ上の承認を終えた後、
ブラウザのアドレスバーに表示されるリダイレクト先の URL を引数として渡し、ログインを完了します。
リダイレクト先は開けなくても問題ありません。URL に含まれる認可コードの有効期限は約 5 分です。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := newEnv()
			flags.apply(cmd, e)
			return runComplete(cmd.Context(), cmd, e, args[0])
		},
	}
	loginCmd.Flags().Bool("browser", false, "OAuth2 authorization code flow (PKCE) でトークンを取得する")
	loginCmd.Flags().Bool("manual", false, "ブラウザを開かず認可 URL のみを出力する (traq auth complete と組み合わせる)")
	loginCmd.Flags().String("scope", "read", "OAuth2 スコープ (read または read,write)")
	authCmd.AddCommand(loginCmd, completeCmd, logoutCmd, statusCmd)
	return authCmd
}

func runLogin(cmd *cobra.Command, newEnv func() *Env, flags *globalFlags) error {
	e := newEnv()
	flags.apply(cmd, e)
	browser, _ := cmd.Flags().GetBool("browser")
	manual, _ := cmd.Flags().GetBool("manual")
	scopeFlag, _ := cmd.Flags().GetString("scope")

	if cmd.Flags().Changed("token") {
		if manual {
			return &ExitCodeError{exitUsage, errors.New("cannot combine --token with --manual")}
		}
		return loginWithToken(cmd.Context(), cmd.OutOrStdout(), e, e.TokenOverride)
	}
	if manual {
		if browser {
			return &ExitCodeError{exitUsage, errors.New("cannot combine --browser with --manual")}
		}
		return runManualLogin(cmd.Context(), e, scopeFlag, cmd.OutOrStdout())
	}
	if !browser {
		return &ExitCodeError{
			exitUsage,
			errors.New("specify either --browser, --manual or --token <TOKEN>"),
		}
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
	return finishLogin(ctx, e, out, code, redirectURI, pk.Verifier, scopes)
}

// finishLogin は認可コードをトークンに交換し、/users/me で検証して保存する。
// ブラウザフロー (login --browser) と手動フロー (traq auth complete) の共通後半処理。
func finishLogin(
	ctx context.Context,
	e *Env,
	out io.Writer,
	code, redirectURI, verifier string,
	fallbackScopes []string,
) error {
	resp, err := auth.ExchangeCode(ctx, e.Origin, e.ClientID, redirectURI, code, verifier)
	if err != nil {
		return err
	}
	scopeList := auth.ParseScopes(resp.Scope)
	if len(scopeList) == 0 {
		scopeList = fallbackScopes
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

func runManualLogin(_ context.Context, e *Env, scopeFlag string, out io.Writer) error {
	if e.ClientID == "" {
		return &ExitCodeError{
			exitUsage,
			errors.New(
				"OAuth client ID is not configured; set TRAQ_CLIENT_ID or --client-id (see docs/auth.md)",
			),
		}
	}
	scopes, err := auth.NormalizeScopes(auth.ParseScopes(scopeFlag))
	if err != nil {
		return &ExitCodeError{exitUsage, err}
	}
	pk, err := auth.NewPKCE()
	if err != nil {
		return err
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
	if _, err := e.sessions().Save(sess); err != nil {
		return err
	}
	authorizeURL := auth.AuthorizeURL(sess.Origin, sess.ClientID, sess.RedirectURI, scopes, pk)
	if _, err := fmt.Fprintln(
		out,
		"Open this URL in a browser to authorize traq-cli:",
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, authorizeURL); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		out,
		"The page after authorization will fail to load; this is expected.\n"+
			"Copy the URL from the browser's address bar and run:\n"+
			"  traq auth complete \"<URL>\"\n"+
			"(the one-time code is valid for about 5 minutes; the pending login expires in %s)\n",
		auth.ManualSessionTTL,
	); err != nil {
		return err
	}
	return nil
}

// parseCallback は traq auth complete の引数から code と state を取り出す。
// 完全な URL のほか、"?code=..." や "code=...&state=..." のようなクエリ文字列も受け付ける。
// 形式が不正なら err を (usage エラー)、ユーザーが拒否したなら denied を返す。
func parseCallback(raw string) (code, state string, denied, err error) {
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

func runComplete(ctx context.Context, cmd *cobra.Command, e *Env, callback string) error {
	code, state, denied, err := parseCallback(callback)
	if err != nil {
		return &ExitCodeError{exitUsage, err}
	}
	if denied != nil {
		return &ExitCodeError{exitGeneral, denied}
	}
	sess, found, err := e.sessions().Load(time.Now())
	if err != nil {
		return err
	}
	if !found {
		return &ExitCodeError{
			exitAuthRequired,
			errors.New("no pending manual login: run 'traq auth login --manual' first"),
		}
	}
	if state != "" && state != sess.State {
		return &ExitCodeError{exitGeneral, errors.New(
			"authorization callback state mismatch; " +
				"re-run 'traq auth login --manual' and use the new URL",
		)}
	}

	sessEnv := *e
	sessEnv.Origin = sess.Origin
	sessEnv.ClientID = sess.ClientID
	if err := finishLogin(
		ctx,
		&sessEnv,
		cmd.OutOrStdout(),
		code,
		sess.RedirectURI,
		sess.Verifier,
		sess.Scopes,
	); err != nil {
		return err
	}
	// 認可コードは 1 回限りなので、トークン保存まで成功したらセッションを消す
	if _, err := e.sessions().Delete(); err != nil {
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
