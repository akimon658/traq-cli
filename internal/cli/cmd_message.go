package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
)

const (
	minSendArgs = 2

	defaultListLimit = 10
)

func newMessageCmd(newEnv func() *Env, flags *globalFlags) *cobra.Command {
	messageCmd := &cobra.Command{
		Use:   "message",
		Short: "メッセージ操作サブコマンド",
	}
	sendMessageCmd := &cobra.Command{
		Use:   "send <チャンネル> <本文>...",
		Short: "チャンネルにメッセージを投稿する",
		Long: `<チャンネル> はチャンネル UUID、または "#general/a/b" 形式のチャンネルパスで指定します。
"#" は省略できます。`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MinimumNArgs(minSendArgs)(cmd, args); err != nil {
				return &ExitCodeError{exitUsage, err}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			e := newEnv()
			flags.apply(cmd, e)
			rt, err := e.resolveToken(cmd.Context())
			if err != nil {
				return err
			}
			if err := e.requireScope(&rt, auth.ScopeWrite); err != nil {
				return err
			}
			c := client.New(e.Origin)
			ctx := cmd.Context()
			channelID, err := resolveChannel(e, &rt, c, ctx, args[0])
			if err != nil {
				return err
			}
			content := strings.Join(args[1:], " ")
			var msg *traq.Message
			err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				msg, resp, err = c.API.MessageAPI.
					PostMessage(ctx, channelID).
					PostMessageRequest(traq.PostMessageRequest{Content: content}).
					Execute()
				return resp, err
			})
			if err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), msg)
		},
	}
	messageCmd.AddCommand(sendMessageCmd, newMessageListCmd(newEnv, flags))
	return messageCmd
}

func newMessageListCmd(newEnv func() *Env, flags *globalFlags) *cobra.Command {
	var (
		limit      int
		offset     int
		sinceStr   string
		untilStr   string
		inclusive  bool
		order      string
		showUnread bool
	)
	listCmd := &cobra.Command{
		Use:   "list <チャンネル>",
		Short: "チャンネルのメッセージ一覧を取得する",
		Long: `<チャンネル> はチャンネル UUID、または "#general/a/b" 形式のチャンネルパスで指定します。
"#" は省略できます。

メッセージは常に古い順 (時系列) に表示されますが、取得順は --order で指定できます。
既定は --order desc (新しい順) のため、初期状態では最新の --limit 件を取得して表示します。
さらに古いメッセージは --offset を進めてください (offset は取得順が基準です)。
例: 最新10件の次に古い10件は --offset 10`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return &ExitCodeError{exitUsage, err}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMessageList(cmd, newEnv, flags, args[0], listOptions{
				limit:      limit,
				offset:     offset,
				since:      sinceStr,
				until:      untilStr,
				inclusive:  inclusive,
				order:      order,
				showUnread: showUnread,
			})
		},
	}
	listCmd.Flags().IntVar(&limit, "limit", defaultListLimit, "取得する件数")
	listCmd.Flags().IntVar(&offset, "offset", 0, "取得順で読み飛ばす件数")
	listCmd.Flags().StringVar(&sinceStr, "since", "", "取得する時間範囲の開始 (RFC3339)")
	listCmd.Flags().StringVar(&untilStr, "until", "", "取得する時間範囲の終了 (RFC3339)")
	listCmd.Flags().BoolVar(&inclusive, "inclusive", false, "時間範囲の端を含めるかどうか")
	listCmd.Flags().StringVar(&order, "order", "desc", "取得する順序 (asc または desc)")
	listCmd.Flags().BoolVar(&showUnread, "show-unread", true, "「ここから未読」の位置を表示する")
	return listCmd
}

type listOptions struct {
	limit      int
	offset     int
	since      string
	until      string
	inclusive  bool
	order      string
	showUnread bool
}

func runMessageList(
	cmd *cobra.Command,
	newEnv func() *Env,
	flags *globalFlags,
	target string,
	opts listOptions,
) error {
	if opts.limit < 1 {
		return &ExitCodeError{exitUsage, fmt.Errorf("--limit must be at least 1")}
	}
	if opts.offset < 0 {
		return &ExitCodeError{exitUsage, fmt.Errorf("--offset must not be negative")}
	}
	if opts.order != "asc" && opts.order != "desc" {
		return &ExitCodeError{exitUsage, fmt.Errorf(`--order must be "asc" or "desc"`)}
	}
	since, err := parseRFC3339Flag("since", opts.since)
	if err != nil {
		return err
	}
	until, err := parseRFC3339Flag("until", opts.until)
	if err != nil {
		return err
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
	channelID, err := resolveChannel(e, &rt, c, ctx, target)
	if err != nil {
		return err
	}

	var msgs []traq.Message
	err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		msgs, resp, err = c.GetChannelMessages(ctx, channelID, client.MessageQuery{
			Limit:     int32(opts.limit),
			Offset:    int32(opts.offset),
			Since:     since,
			Until:     until,
			Inclusive: opts.inclusive,
			Order:     opts.order,
		})
		return resp, err
	})
	if err != nil {
		return err
	}

	userByID := map[string]traq.User{}
	stampNames := map[string]string{}
	if len(msgs) > 0 {
		var users []traq.User
		err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			users, resp, err = c.GetUsers(ctx)
			return resp, err
		})
		if err != nil {
			return err
		}
		for _, u := range users {
			userByID[u.Id] = u
		}

		var stamps []traq.StampWithThumbnail
		err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			stamps, resp, err = c.GetStamps(ctx)
			return resp, err
		})
		if err != nil {
			return err
		}
		for _, s := range stamps {
			stampNames[s.Id] = s.Name
		}
	}

	var unread *traq.UnreadChannel
	if opts.showUnread {
		// 未読情報は表示の補助なので、取得に失敗しても本体の表示は続行する。
		var list []traq.UnreadChannel
		uerr := e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			list, resp, apiErr = c.GetMyUnreadChannels(ctx)
			return resp, apiErr
		})
		if uerr == nil {
			for i := range list {
				if list[i].ChannelId == channelID {
					unread = &list[i]
					break
				}
			}
		}
	}

	return writeMessageList(cmd.OutOrStdout(), messageList{
		label:  channelLabel(target),
		limit:  opts.limit,
		offset: opts.offset,
		order:  opts.order,
		msgs:   msgs,
		users:  userByID,
		stamps: stampNames,
		loc:    time.Local,
		unread: unread,
	})
}

// channelLabel は表示用のチャンネル名を組み立てる。UUID 指定ならそのまま、
// パス指定なら先頭に "#" を付ける。
func channelLabel(target string) string {
	if _, err := uuid.Parse(target); err == nil {
		return target
	}
	return "#" + strings.TrimPrefix(target, "#")
}

func parseRFC3339Flag(name, value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, &ExitCodeError{exitUsage, fmt.Errorf(
			"invalid --%s: %v (use RFC3339, e.g. 2026-01-02T15:04:05Z)", name, err,
		)}
	}
	return &t, nil
}

func resolveChannel(
	e *Env,
	rt *ResolvedToken,
	c *client.Client,
	ctx context.Context,
	target string,
) (string, error) {
	if _, err := uuid.Parse(target); err == nil {
		return target, nil
	}
	if rt.Token.LacksScope(auth.ScopeRead) {
		return "", e.requireScope(rt, auth.ScopeRead)
	}
	path := strings.TrimPrefix(target, "#")
	var channelID string
	err := e.apiExec(ctx, rt, func(ctx context.Context) (*http.Response, error) {
		list, resp, apiErr := c.API.ChannelAPI.GetChannels(ctx).Execute()
		if apiErr != nil {
			return resp, apiErr
		}
		channelID, apiErr = findChannelByPath(list, path)
		return resp, apiErr
	})
	if err != nil {
		return "", err
	}
	if channelID == "" {
		return "", &ExitCodeError{exitGeneral, fmt.Errorf("channel not found: %s", target)}
	}
	return channelID, nil
}

func findChannelByPath(list *traq.ChannelList, path string) (string, error) {
	byID := make(map[string]traq.Channel, len(list.Public))
	parent := make(map[string]string, len(list.Public))
	for _, ch := range list.Public {
		byID[ch.Id] = ch
		parent[ch.Id] = ""
		if p := ch.ParentId.Get(); p != nil {
			parent[ch.Id] = *p
		}
	}
	full := make(map[string]string, len(list.Public))
	for id := range byID {
		var segs []string
		cur := id
		for i := 0; i <= len(byID); i++ {
			ch, ok := byID[cur]
			if !ok {
				segs = nil
				break
			}
			segs = append([]string{ch.Name}, segs...)
			cur = parent[cur]
			if cur == "" || cur == id {
				break
			}
		}
		if segs == nil {
			continue
		}
		full[strings.Join(segs, "/")] = id
	}
	if id, ok := full[path]; ok {
		return id, nil
	}
	return "", fmt.Errorf("channel not found: %s", path)
}
