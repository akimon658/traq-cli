package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/authflow"
	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
	"github.com/akimon658/traq-cli/internal/session"
	"github.com/akimon658/traq-cli/internal/view"
)

func newAuthCmd(newEnv func() *config.Env, flags *globalFlags) *cobra.Command {
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

func runLogin(cmd *cobra.Command, newEnv func() *config.Env, flags *globalFlags) error {
	e := newEnv()
	flags.apply(cmd, e)
	browser, _ := cmd.Flags().GetBool("browser")
	manual, _ := cmd.Flags().GetBool("manual")
	scopeFlag, _ := cmd.Flags().GetString("scope")

	if cmd.Flags().Changed("token") {
		if manual {
			return exit.UsageError(errors.New("cannot combine --token with --manual"))
		}
		c := client.New(e.Origin)
		t, venue, err := authflow.LoginWithToken(cmd.Context(), e, c, e.TokenOverride)
		if err != nil {
			return err
		}
		return view.WriteLoginSuccess(cmd.OutOrStdout(), t, venue)
	}
	if manual {
		if browser {
			return exit.UsageError(errors.New("cannot combine --browser with --manual"))
		}
		return runManualLogin(e, scopeFlag, cmd.OutOrStdout())
	}
	if !browser {
		return exit.UsageError(errors.New("specify either --browser, --manual or --token <TOKEN>"))
	}
	return runBrowserLogin(cmd.Context(), e, scopeFlag, openBrowser, cmd.OutOrStdout())
}

func runBrowserLogin(
	ctx context.Context,
	e *config.Env,
	scopeFlag string,
	openBrowser func(string),
	out io.Writer,
) error {
	sess, err := authflow.PrepareBrowserLogin(e, scopeFlag)
	if err != nil {
		return err
	}
	defer sess.Callback.Close()
	if err := view.WriteBrowserInstructions(out, sess.AuthorizeURL); err != nil {
		return err
	}
	openBrowser(sess.AuthorizeURL)
	ctx, cancel := context.WithTimeout(ctx, authflow.LoginTimeout)
	defer cancel()
	code, err := sess.Callback.Wait(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			err = errors.New("timed out waiting for authorization")
		}
		return err
	}
	c := client.New(e.Origin)
	t, venue, err := authflow.FinishLogin(
		ctx, e, c, code, sess.RedirectURI, sess.PKCE.Verifier, sess.Scopes,
	)
	if err != nil {
		return err
	}
	return view.WriteLoginSuccessWithScopes(out, t, venue)
}

func runManualLogin(e *config.Env, scopeFlag string, out io.Writer) error {
	authorizeURL, err := authflow.StartManualLogin(e, scopeFlag)
	if err != nil {
		return err
	}
	return view.WriteManualInstructions(out, authorizeURL, auth.ManualSessionTTL)
}

func runComplete(ctx context.Context, cmd *cobra.Command, e *config.Env, callback string) error {
	c := client.New(e.Origin)
	t, venue, err := authflow.CompleteManualLogin(ctx, e, c, callback)
	if err != nil {
		return err
	}
	return view.WriteLoginSuccessWithScopes(cmd.OutOrStdout(), t, venue)
}

func runLogout(cmd *cobra.Command, e *config.Env) error {
	warning, err := authflow.Logout(cmd.Context(), e)
	if err != nil {
		return err
	}
	if warning != nil {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "failed to revoke the token on the server:", warning)
	}
	return view.WriteLogout(cmd.OutOrStdout())
}

func runStatus(cmd *cobra.Command, e *config.Env) error {
	rt, err := session.ResolveToken(cmd.Context(), e)
	if err != nil {
		return err
	}
	c := client.New(e.Origin)
	me, _, meErr := authflow.VerifyToken(cmd.Context(), c, rt.Token.AccessToken)
	if me != nil {
		rt.Token.UserID = me.Id
		rt.Token.UserName = me.Name
		rt.Token.UserBot = me.Bot
	}
	if err := view.WriteAuthStatus(cmd.OutOrStdout(), e.Origin, rt.Source, *rt.Token); err != nil {
		return err
	}
	return meErr
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
