package cli

import (
	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/domain/user"
	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/ui/view"
)

func newUserCmd(newEnv func() *config.Env, flags *globalFlags) *cobra.Command {
	userCmd := &cobra.Command{
		Use:     "user",
		Aliases: []string{"users"},
		Short:   "ユーザー操作サブコマンド",
	}
	userMeCmd := &cobra.Command{
		Use:   "me",
		Short: "自分のユーザー詳細を取得する",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return exit.UsageError(err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			e := newEnv()
			flags.apply(cmd, e)
			rt, err := auth.ResolveToken(
				cmd.Context(),
				e.Origin,
				e.ClientID,
				e.TokenOverride,
				e.AccessToken,
				e.LoadStore(),
			)
			if err != nil {
				return err
			}
			c := client.New(e.Origin)
			me, err := user.FetchMe(cmd.Context(), e, &rt, c)
			if err != nil {
				return err
			}
			return view.WriteMyUserDetail(cmd.OutOrStdout(), me, nil)
		},
	}
	userCmd.AddCommand(userMeCmd)
	return userCmd
}
