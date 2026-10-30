package cli

import (
	"github.com/moznion/go-optional"
	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
	"github.com/akimon658/traq-cli/internal/message"
	"github.com/akimon658/traq-cli/internal/session"
	"github.com/akimon658/traq-cli/internal/view"
)

func newMessageSearchCmd(newEnv func() *config.Env, flags *globalFlags) *cobra.Command {
	var (
		in             string
		from           []string
		to             []string
		afterStr       string
		beforeStr      string
		citation       string
		bot            bool
		hasURL         bool
		hasAttachments bool
		hasImage       bool
		hasVideo       bool
		hasAudio       bool
		limit          int
		offset         int
		sort           string
	)
	searchCmd := &cobra.Command{
		Use:   "search [word]",
		Short: "メッセージを検索する",
		Long: `[word] は Simple-Query-String-Syntax で解釈されます (省略可)。
例: "phrase match" +(foo|bar) -baz

--in はチャンネル UUID、または "#general/a/b" 形式のチャンネルパスで指定します ("#" は省略可)。
--from / --to はユーザー UUID、または "@name" / "name" 形式で指定します ("@" は省略可)。
--after / --before は RFC3339 形式 (例: 2026-01-02T15:04:05Z) で指定します。
真偽値フィルタ (--bot 等) は指定したときのみ有効になり、--bot=false のように明示すると偽のものだけに絞ります。`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
				return exit.UsageError(err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var word string
			if len(args) > 0 {
				word = args[0]
			}
			e := newEnv()
			flags.apply(cmd, e)
			opts := message.SearchOptions{
				In:             in,
				From:           from,
				To:             to,
				After:          afterStr,
				Before:         beforeStr,
				Citation:       citation,
				Bot:            changedBoolFlag(cmd, "bot", bot),
				HasURL:         changedBoolFlag(cmd, "has-url", hasURL),
				HasAttachments: changedBoolFlag(cmd, "has-attachments", hasAttachments),
				HasImage:       changedBoolFlag(cmd, "has-image", hasImage),
				HasVideo:       changedBoolFlag(cmd, "has-video", hasVideo),
				HasAudio:       changedBoolFlag(cmd, "has-audio", hasAudio),
				Limit:          limit,
				Offset:         offset,
				Sort:           sort,
			}
			if err := message.ValidateSearchOptions(word, opts); err != nil {
				return err
			}
			rt, err := session.ResolveToken(cmd.Context(), e)
			if err != nil {
				return err
			}
			c := client.New(e.Origin)
			res, err := message.Search(cmd.Context(), e, &rt, c, word, opts)
			if err != nil {
				return err
			}
			return view.WriteMessageList(cmd.OutOrStdout(), view.NewSearchView(
				res.Label, res.Limit, res.Offset, res.Sort, res.TotalHits,
				res.Msgs, res.Users, res.Stamps, res.Loc, res.Origin, res.Quoted, res.Files,
			))
		},
	}
	searchCmd.Flags().StringVar(&in, "in", "", "検索対象チャンネル (UUID または #パス)")
	searchCmd.Flags().StringSliceVar(&from, "from", nil, "投稿者 (UUID または @name、カンマ区切り・繰り返し可)")
	searchCmd.Flags().StringSliceVar(&to, "to", nil, "メンション先 (UUID または @name、カンマ区切り・繰り返し可)")
	searchCmd.Flags().StringVar(&afterStr, "after", "", "投稿日時の下限 (RFC3339)")
	searchCmd.Flags().StringVar(&beforeStr, "before", "", "投稿日時の上限 (RFC3339)")
	searchCmd.Flags().StringVar(&citation, "citation", "", "引用元メッセージ UUID")
	searchCmd.Flags().BoolVar(&bot, "bot", false, "bot の投稿に絞る (false 指定で人間だけに絞る)")
	searchCmd.Flags().BoolVar(&hasURL, "has-url", false, "URL 含む投稿に絞る")
	searchCmd.Flags().BoolVar(&hasAttachments, "has-attachments", false, "添付ファイル付き投稿に絞る")
	searchCmd.Flags().BoolVar(&hasImage, "has-image", false, "画像付き投稿に絞る")
	searchCmd.Flags().BoolVar(&hasVideo, "has-video", false, "動画付き投稿に絞る")
	searchCmd.Flags().BoolVar(&hasAudio, "has-audio", false, "音声付き投稿に絞る")
	searchCmd.Flags().IntVar(&limit, "limit", message.DefaultSearchLimit, "取得する件数 (1-100)")
	searchCmd.Flags().IntVar(&offset, "offset", 0, "読み飛ばす件数 (0-9900)")
	searchCmd.Flags().
		StringVar(&sort, "sort", message.DefaultSearchSort, "取得する順序 (createdAt,-createdAt,updatedAt,-updatedAt)")
	return searchCmd
}

// changedBoolFlag はフラグが指定されたときのみ Some を返す。
func changedBoolFlag(cmd *cobra.Command, name string, value bool) optional.Option[bool] {
	if !cmd.Flags().Changed(name) {
		return optional.None[bool]()
	}
	return optional.Some(value)
}
