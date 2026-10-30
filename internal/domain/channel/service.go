package channel

import (
	"context"
	"net/http"
	"strings"
	"uuid"

	"github.com/traPtitech/go-traq"
	"golang.org/x/sync/errgroup"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// ResolveChannel はチャンネル指定 (UUID または "#general/a/b" 形式のパス) を ID に解決する。
func ResolveChannel(
	ctx context.Context,
	e *config.Env,
	rt *auth.ResolvedToken,
	c *client.Client,
	target string,
) (string, error) {
	if _, err := uuid.Parse(target); err == nil {
		return target, nil
	}
	if rt.Token.LacksScope(auth.ScopeRead) {
		return "", auth.RequireScope(rt, auth.ScopeRead)
	}
	path := strings.TrimPrefix(target, "#")
	var channelID string
	err := auth.Exec(
		ctx,
		e.Origin,
		e.ClientID,
		e.LoadStore(),
		rt,
		func(ctx context.Context) (*http.Response, error) {
			list, resp, apiErr := c.GetChannels(ctx, true)
			if apiErr != nil {
				return resp, apiErr
			}
			channelID, apiErr = FindByPath(list, path)
			return resp, apiErr
		},
	)
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
	rt *auth.ResolvedToken,
	c *client.Client,
) (Groups, error) {
	if err := auth.RequireScope(rt, auth.ScopeRead); err != nil {
		return Groups{}, err
	}
	var unreads []traq.UnreadChannel
	if err := auth.Exec(
		ctx,
		e.Origin,
		e.ClientID,
		e.LoadStore(),
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			unreads, resp, apiErr = c.GetMyUnreadChannels(ctx)
			return resp, apiErr
		},
	); err != nil {
		return Groups{}, err
	}
	if len(unreads) == 0 {
		return Groups{}, nil
	}

	// 最初の取得でトークンは検証/更新済みのため、残りの独立した3取得は並列化する。
	// 万が一の401リトライ時の競合を避けるため、各 goroutine はトークンのコピーを参照する。
	var channelList *traq.ChannelList
	var subs []traq.UserSubscribeState
	var starIDs []string
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		tok := *rt.Token
		rtLocal := &auth.ResolvedToken{Token: &tok, Source: rt.Source}
		return auth.Exec(
			gctx,
			e.Origin,
			e.ClientID,
			e.LoadStore(),
			rtLocal,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				channelList, resp, apiErr = c.GetChannels(ctx, true)
				return resp, apiErr
			},
		)
	})
	g.Go(func() error {
		tok := *rt.Token
		rtLocal := &auth.ResolvedToken{Token: &tok, Source: rt.Source}
		return auth.Exec(
			gctx,
			e.Origin,
			e.ClientID,
			e.LoadStore(),
			rtLocal,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				subs, resp, apiErr = c.GetMyChannelSubscriptions(ctx)
				return resp, apiErr
			},
		)
	})
	g.Go(func() error {
		tok := *rt.Token
		rtLocal := &auth.ResolvedToken{Token: &tok, Source: rt.Source}
		return auth.Exec(
			gctx,
			e.Origin,
			e.ClientID,
			e.LoadStore(),
			rtLocal,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				starIDs, resp, apiErr = c.GetMyStars(ctx)
				return resp, apiErr
			},
		)
	})
	if err := g.Wait(); err != nil {
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
		if err := auth.Exec(
			ctx,
			e.Origin,
			e.ClientID,
			e.LoadStore(),
			rt,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				users, resp, apiErr = c.GetUsers(ctx)
				return resp, apiErr
			},
		); err != nil {
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
