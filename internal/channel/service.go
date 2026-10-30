package channel

import (
	"context"
	"net/http"
	"strings"
	"uuid"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
	"github.com/akimon658/traq-cli/internal/session"
)

// ResolveChannel はチャンネル指定 (UUID または "#general/a/b" 形式のパス) を ID に解決する。
func ResolveChannel(
	ctx context.Context,
	e *config.Env,
	rt *session.ResolvedToken,
	c *client.Client,
	target string,
) (string, error) {
	if _, err := uuid.Parse(target); err == nil {
		return target, nil
	}
	if rt.Token.LacksScope(auth.ScopeRead) {
		return "", session.RequireScope(rt, auth.ScopeRead)
	}
	path := strings.TrimPrefix(target, "#")
	var channelID string
	err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		list, resp, apiErr := c.API.ChannelAPI.GetChannels(ctx).Execute()
		if apiErr != nil {
			return resp, apiErr
		}
		channelID, apiErr = FindByPath(list, path)
		return resp, apiErr
	})
	if err != nil {
		return "", err
	}
	if channelID == "" {
		return "", exit.Generalf("channel not found: %s", target)
	}
	return channelID, nil
}

// FetchUnreadGroups は未読情報を取得し、表示用のグループに分類して返す。
func FetchUnreadGroups(
	ctx context.Context,
	e *config.Env,
	rt *session.ResolvedToken,
	c *client.Client,
) (Groups, error) {
	var unreads []traq.UnreadChannel
	if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		var apiErr error
		unreads, resp, apiErr = c.GetMyUnreadChannels(ctx)
		return resp, apiErr
	}); err != nil {
		return Groups{}, err
	}
	if len(unreads) == 0 {
		return Groups{}, nil
	}

	var channelList *traq.ChannelList
	if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		var apiErr error
		channelList, resp, apiErr = c.GetChannels(ctx, true)
		return resp, apiErr
	}); err != nil {
		return Groups{}, err
	}

	var subs []traq.UserSubscribeState
	if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		var apiErr error
		subs, resp, apiErr = c.GetMyChannelSubscriptions(ctx)
		return resp, apiErr
	}); err != nil {
		return Groups{}, err
	}

	var starIDs []string
	if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		var apiErr error
		starIDs, resp, apiErr = c.GetMyStars(ctx)
		return resp, apiErr
	}); err != nil {
		return Groups{}, err
	}

	pathMap := BuildPathMap(channelList)
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
		if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			users, resp, apiErr = c.GetUsers(ctx)
			return resp, apiErr
		}); err != nil {
			return Groups{}, err
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

	groups, err := Classify(unreads, pathMap, dmMap, userNames, subLevels, stars)
	if err != nil {
		return Groups{}, exit.GeneralError(err)
	}
	return groups, nil
}
