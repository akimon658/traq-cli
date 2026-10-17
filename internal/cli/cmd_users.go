package cli

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
)

func newUsersCmd(newEnv func() *Env, flags *globalFlags) *cobra.Command {
	usersCmd := &cobra.Command{
		Use:   "users",
		Short: "ユーザー操作サブコマンド",
	}
	usersMeCmd := &cobra.Command{
		Use:   "me",
		Short: "自分のユーザー詳細を取得する",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return &ExitCodeError{exitUsage, err}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			e := newEnv()
			flags.apply(cmd, e)
			rt, err := e.resolveToken(cmd.Context())
			if err != nil {
				return err
			}
			if err := e.requireScope(&rt, auth.ScopeRead); err != nil {
				return err
			}
			c := client.New(e.Origin)
			var me *traq.MyUserDetail
			err = e.apiExec(
				cmd.Context(),
				&rt,
				func(ctx context.Context) (*http.Response, error) {
					var resp *http.Response
					me, resp, err = c.GetMe(ctx, rt.Token.AccessToken)
					return resp, err
				},
			)
			if err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), me)
		},
	}
	usersCmd.AddCommand(usersMeCmd)
	return usersCmd
}
