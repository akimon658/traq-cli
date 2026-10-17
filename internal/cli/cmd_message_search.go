package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/spf13/cobra"
	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
)

const (
	defaultSearchLimit = 10
	maxSearchLimit     = 100
	maxSearchOffset    = 9900
	defaultSearchSort  = "-createdAt"
	searchSortCreated  = "createdAt"
	searchSortCreatedR = "-createdAt"
	searchSortUpdated  = "updatedAt"
	searchSortUpdatedR = "-updatedAt"
)

func newMessageSearchCmd(newEnv func() *Env, flags *globalFlags) *cobra.Command {
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
				return &ExitCodeError{exitUsage, err}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var word string
			if len(args) > 0 {
				word = args[0]
			}
			return runMessageSearch(cmd, newEnv, flags, word, searchOptions{
				in:             in,
				from:           from,
				to:             to,
				after:          afterStr,
				before:         beforeStr,
				citation:       citation,
				botChanged:     cmd.Flags().Changed("bot"),
				bot:            bot,
				hasURLChanged:  cmd.Flags().Changed("has-url"),
				hasURL:         hasURL,
				hasAttChanged:  cmd.Flags().Changed("has-attachments"),
				hasAttachments: hasAttachments,
				hasImgChanged:  cmd.Flags().Changed("has-image"),
				hasImage:       hasImage,
				hasVidChanged:  cmd.Flags().Changed("has-video"),
				hasVideo:       hasVideo,
				hasAudChanged:  cmd.Flags().Changed("has-audio"),
				hasAudio:       hasAudio,
				limit:          limit,
				offset:         offset,
				sort:           sort,
			})
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
	searchCmd.Flags().IntVar(&limit, "limit", defaultSearchLimit, "取得する件数 (1-100)")
	searchCmd.Flags().IntVar(&offset, "offset", 0, "読み飛ばす件数 (0-9900)")
	searchCmd.Flags().
		StringVar(&sort, "sort", defaultSearchSort, "取得する順序 (createdAt,-createdAt,updatedAt,-updatedAt)")
	return searchCmd
}

type searchOptions struct {
	in             string
	from           []string
	to             []string
	after          string
	before         string
	citation       string
	botChanged     bool
	bot            bool
	hasURLChanged  bool
	hasURL         bool
	hasAttChanged  bool
	hasAttachments bool
	hasImgChanged  bool
	hasImage       bool
	hasVidChanged  bool
	hasVideo       bool
	hasAudChanged  bool
	hasAudio       bool
	limit          int
	offset         int
	sort           string
}

func runMessageSearch(
	cmd *cobra.Command,
	newEnv func() *Env,
	flags *globalFlags,
	word string,
	opts searchOptions,
) error {
	if opts.limit < 1 || opts.limit > maxSearchLimit {
		return &ExitCodeError{
			exitUsage,
			fmt.Errorf("--limit must be between 1 and %d", maxSearchLimit),
		}
	}
	if opts.offset < 0 || opts.offset > maxSearchOffset {
		return &ExitCodeError{
			exitUsage,
			fmt.Errorf("--offset must be between 0 and %d", maxSearchOffset),
		}
	}
	switch opts.sort {
	case searchSortCreated, searchSortCreatedR, searchSortUpdated, searchSortUpdatedR:
	default:
		return &ExitCodeError{exitUsage, fmt.Errorf(
			`--sort must be one of "createdAt", "-createdAt", "updatedAt", "-updatedAt"`,
		)}
	}
	after, err := parseRFC3339Flag("after", opts.after)
	if err != nil {
		return err
	}
	before, err := parseRFC3339Flag("before", opts.before)
	if err != nil {
		return err
	}
	if opts.citation != "" {
		if _, err := uuid.Parse(opts.citation); err != nil {
			return &ExitCodeError{exitUsage, fmt.Errorf("invalid --citation: %v", err)}
		}
	}

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
	ctx := cmd.Context()

	// ユーザー一覧は名前解決と表示の両方に使うため先に取得する。
	var users []traq.User
	err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		users, resp, err = c.GetUsers(ctx)
		return resp, err
	})
	if err != nil {
		return err
	}
	userByID := make(map[string]traq.User, len(users))
	nameToID := make(map[string]string, len(users))
	for _, u := range users {
		userByID[u.Id] = u
		nameToID[u.Name] = u.Id
	}

	q := client.SearchQuery{
		Word:   word,
		After:  after,
		Before: before,
		Limit:  int32(opts.limit),
		Offset: int32(opts.offset),
		Sort:   opts.sort,
	}
	if opts.in != "" {
		channelID, err := resolveChannel(e, &rt, c, ctx, opts.in)
		if err != nil {
			return err
		}
		q.In = channelID
		q.InSet = true
	}
	if len(cleanUserInputs(opts.from)) > 0 {
		ids, err := resolveUserIDs(cleanUserInputs(opts.from), nameToID)
		if err != nil {
			return &ExitCodeError{exitUsage, fmt.Errorf("invalid --from: %v", err)}
		}
		q.From = ids
		q.FromSet = true
	}
	if len(cleanUserInputs(opts.to)) > 0 {
		ids, err := resolveUserIDs(cleanUserInputs(opts.to), nameToID)
		if err != nil {
			return &ExitCodeError{exitUsage, fmt.Errorf("invalid --to: %v", err)}
		}
		q.To = ids
		q.ToSet = true
	}
	if opts.citation != "" {
		q.Citation = opts.citation
		q.CitationSet = true
	}
	if opts.botChanged {
		b := opts.bot
		q.Bot = &b
	}
	if opts.hasURLChanged {
		b := opts.hasURL
		q.HasURL = &b
	}
	if opts.hasAttChanged {
		b := opts.hasAttachments
		q.HasAttachments = &b
	}
	if opts.hasImgChanged {
		b := opts.hasImage
		q.HasImage = &b
	}
	if opts.hasVidChanged {
		b := opts.hasVideo
		q.HasVideo = &b
	}
	if opts.hasAudChanged {
		b := opts.hasAudio
		q.HasAudio = &b
	}

	var result *traq.MessageSearchResult
	err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		result, resp, err = c.SearchMessages(ctx, q)
		return resp, err
	})
	if err != nil {
		return err
	}

	var stamps []traq.StampWithThumbnail
	if len(result.Hits) > 0 {
		err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			stamps, resp, err = c.GetStamps(ctx)
			return resp, err
		})
		if err != nil {
			return err
		}
	}
	stampNames := make(map[string]string, len(stamps))
	for _, s := range stamps {
		stampNames[s.Id] = s.Name
	}

	total := result.TotalHits
	return writeMessageList(cmd.OutOrStdout(), messageList{
		label:     searchLabel(word, opts.in),
		limit:     opts.limit,
		offset:    opts.offset,
		msgs:      result.Hits,
		users:     userByID,
		stamps:    stampNames,
		loc:       time.Local,
		totalHits: &total,
		sort:      opts.sort,
	})
}

// searchLabel は検索結果の表示用ラベルを組み立てる。
func searchLabel(word, in string) string {
	var b strings.Builder
	b.WriteString("search")
	if word != "" {
		fmt.Fprintf(&b, " %q", word)
	}
	if in != "" {
		fmt.Fprintf(&b, " in %s", channelLabel(in))
	}
	return b.String()
}

// cleanUserInputs は StringSlice の空要素を取り除く。
func cleanUserInputs(inputs []string) []string {
	out := make([]string, 0, len(inputs))
	for _, s := range inputs {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// resolveUserIDs は UUID または @name/name を UUID に変換する。
func resolveUserIDs(inputs []string, nameToID map[string]string) ([]string, error) {
	ids := make([]string, 0, len(inputs))
	for _, in := range inputs {
		if _, err := uuid.Parse(in); err == nil {
			ids = append(ids, in)
			continue
		}
		name := strings.TrimPrefix(in, "@")
		if name == "" {
			return nil, fmt.Errorf("empty user name")
		}
		id, ok := nameToID[name]
		if !ok {
			return nil, fmt.Errorf("user not found: %s", in)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
