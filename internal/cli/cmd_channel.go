package cli

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
)

func newChannelCmd(newEnv func() *Env, flags *globalFlags) *cobra.Command {
	channelCmd := &cobra.Command{
		Use:   "channel",
		Short: "チャンネル操作サブコマンド",
	}
	channelCmd.AddCommand(newChannelUnreadCmd(newEnv, flags))
	return channelCmd
}

func newChannelUnreadCmd(newEnv func() *Env, flags *globalFlags) *cobra.Command {
	unreadCmd := &cobra.Command{
		Use:   "unread",
		Short: "未読チャンネル一覧を取得する",
		Long: `未読チャンネルを DM、メンション、通知オン、お気に入り、その他の順に表示します。
各グループ内は未読の更新が新しい順です。チャンネル名と未読件数を表示します。`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return &ExitCodeError{exitUsage, err}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runChannelUnread(cmd, newEnv, flags)
		},
	}
	return unreadCmd
}

func runChannelUnread(
	cmd *cobra.Command,
	newEnv func() *Env,
	flags *globalFlags,
) error {
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

	var unreads []traq.UnreadChannel
	err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		unreads, resp, err = c.GetMyUnreadChannels(ctx)
		return resp, err
	})
	if err != nil {
		return err
	}
	if len(unreads) == 0 {
		return writeUnreadChannels(cmd.OutOrStdout(), unreadGroups{})
	}

	var channelList *traq.ChannelList
	err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		channelList, resp, err = c.GetChannels(ctx, true)
		return resp, err
	})
	if err != nil {
		return err
	}

	var subs []traq.UserSubscribeState
	err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		subs, resp, err = c.GetMyChannelSubscriptions(ctx)
		return resp, err
	})
	if err != nil {
		return err
	}

	var starIDs []string
	err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		starIDs, resp, err = c.GetMyStars(ctx)
		return resp, err
	})
	if err != nil {
		return err
	}

	pathMap := buildChannelPathMap(channelList)
	dmMap := make(map[string]string, len(channelList.Dm))
	for _, dm := range channelList.Dm {
		dmMap[dm.Id] = dm.UserId
	}

	userNames := map[string]string{}
	needUsers := false
	for _, u := range unreads {
		if _, ok := dmMap[u.ChannelId]; ok {
			needUsers = true
			break
		}
	}
	if needUsers {
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
			userNames[u.Id] = u.Name
		}
	}

	subLevels := make(map[string]traq.ChannelSubscribeLevel, len(subs))
	for _, s := range subs {
		subLevels[s.ChannelId] = s.Level
	}
	stars := make(map[string]bool, len(starIDs))
	for _, id := range starIDs {
		stars[id] = true
	}

	groups, err := classifyUnreads(unreads, pathMap, dmMap, userNames, subLevels, stars)
	if err != nil {
		return &ExitCodeError{exitGeneral, err}
	}
	return writeUnreadChannels(cmd.OutOrStdout(), groups)
}
