package cli

import (
	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/domain/channel"
	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/ui/view"
)

func newChannelCmd(newEnv func() *config.Env, flags *globalFlags) *cobra.Command {
	channelCmd := &cobra.Command{
		Use:     "channel",
		Aliases: []string{"channels"},
		Short:   "チャンネル操作サブコマンド",
	}
	channelCmd.AddCommand(newChannelUnreadCmd(newEnv, flags))
	return channelCmd
}

func newChannelUnreadCmd(newEnv func() *config.Env, flags *globalFlags) *cobra.Command {
	unreadCmd := &cobra.Command{
		Use:   "unread",
		Short: "未読チャンネル一覧を取得する",
		Long: `未読チャンネルを DM、メンション、通知オン、お気に入り、その他の順に表示します。
各グループ内は未読の更新が新しい順です。チャンネル名と未読件数を表示します。`,
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
			groups, err := channel.FetchUnreadGroups(cmd.Context(), e, &rt, c)
			if err != nil {
				return err
			}
			return view.WriteUnreadChannels(cmd.OutOrStdout(), groups)
		},
	}
	return unreadCmd
}
