package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
	"github.com/akimon658/traq-cli/internal/message"
	"github.com/akimon658/traq-cli/internal/session"
	"github.com/akimon658/traq-cli/internal/view"
)

const (
	minSendArgs = 2

	defaultListLimit = 10
)

func newMessageCmd(newEnv func() *config.Env, flags *globalFlags) *cobra.Command {
	messageCmd := &cobra.Command{
		Use:     "message",
		Aliases: []string{"messages"},
		Short:   "メッセージ操作サブコマンド",
	}
	sendMessageCmd := &cobra.Command{
		Use:   "send <チャンネル> <本文>...",
		Short: "チャンネルにメッセージを投稿する",
		Long: `<チャンネル> はチャンネル UUID、または "#general/a/b" 形式のチャンネルパスで指定します。
"#" は省略できます。`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MinimumNArgs(minSendArgs)(cmd, args); err != nil {
				return exit.UsageError(err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			e := newEnv()
			flags.apply(cmd, e)
			rt, err := session.ResolveToken(cmd.Context(), e)
			if err != nil {
				return err
			}
			c := client.New(e.Origin)
			res, err := message.Send(cmd.Context(), e, &rt, c, args[0], strings.Join(args[1:], " "))
			if err != nil {
				return err
			}
			return view.WriteSingleMessage(cmd.OutOrStdout(), view.NewSingleView(
				res.Msg, res.Users, res.Stamps, res.Loc, res.Origin, res.Quoted, res.Files,
			))
		},
	}
	messageCmd.AddCommand(
		sendMessageCmd,
		newMessageListCmd(newEnv, flags),
		newMessageSearchCmd(newEnv, flags),
	)
	return messageCmd
}

func newMessageListCmd(newEnv func() *config.Env, flags *globalFlags) *cobra.Command {
	var (
		limit      int
		offset     int
		sinceStr   string
		untilStr   string
		inclusive  bool
		order      string
		showUnread bool
		unreadOnly bool
	)
	listCmd := &cobra.Command{
		Use:   "list <チャンネル>",
		Short: "チャンネルのメッセージ一覧を取得する",
		Long: `<チャンネル> はチャンネル UUID、または "#general/a/b" 形式のチャンネルパスで指定します。
"#" は省略できます。

メッセージは常に古い順 (時系列) に表示されますが、取得順は --order で指定できます。
既定は --order desc (新しい順) のため、初期状態では最新の --limit 件を取得して表示します。
さらに古いメッセージは --offset を進めてください (offset は取得順が基準です)。
例: 最新10件の次に古い10件は --offset 10

--unread-only を指定すると未読メッセージのみを表示します。
取得範囲は未読の最古日時以降に限定され、--since と併用した場合は新しい方が採用されます。
未読の境界メッセージを必ず表示に含めるため、範囲の端は常に含まれます (--inclusive は無視されます)。
未読が無いときは「未読0件」とだけ表示して終了します。
bot トークンでは未読が生成されないため、常に未読0件になります。`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return exit.UsageError(err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			e := newEnv()
			flags.apply(cmd, e)
			opts := message.ListOptions{
				Limit:      limit,
				Offset:     offset,
				Since:      sinceStr,
				Until:      untilStr,
				Inclusive:  inclusive,
				Order:      order,
				ShowUnread: showUnread,
				UnreadOnly: unreadOnly,
			}
			if err := message.ValidateListOptions(opts); err != nil {
				return err
			}
			rt, err := session.ResolveToken(cmd.Context(), e)
			if err != nil {
				return err
			}
			c := client.New(e.Origin)
			res, err := message.List(cmd.Context(), e, &rt, c, args[0], opts)
			if err != nil {
				return err
			}
			return view.WriteMessageList(cmd.OutOrStdout(), view.NewListView(
				res.Label, res.Limit, res.Offset, res.Order,
				res.Msgs, res.Users, res.Stamps, res.Loc,
				res.Unread, res.UnreadOnly, res.Origin, res.Quoted, res.Files,
			))
		},
	}
	listCmd.Flags().IntVar(&limit, "limit", defaultListLimit, "取得する件数")
	listCmd.Flags().IntVar(&offset, "offset", 0, "取得順で読み飛ばす件数")
	listCmd.Flags().StringVar(&sinceStr, "since", "", "取得する時間範囲の開始 (RFC3339)")
	listCmd.Flags().StringVar(&untilStr, "until", "", "取得する時間範囲の終了 (RFC3339)")
	listCmd.Flags().BoolVar(&inclusive, "inclusive", false, "時間範囲の端を含めるかどうか")
	listCmd.Flags().StringVar(&order, "order", "desc", "取得する順序 (asc または desc)")
	listCmd.Flags().BoolVar(&showUnread, "show-unread", true, "「ここから未読」の位置を表示する")
	listCmd.Flags().BoolVar(
		&unreadOnly,
		"unread-only",
		false,
		"未読メッセージのみ表示する (bot トークンでは常に未読0件になる)",
	)
	return listCmd
}
