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

// Service はチャンネル操作に必要な常駐依存を束ねる。
// 実行環境と通信路は構築時に固定し、要求ごとの認証情報はメソッド引数で受ける。
type Service struct {
	env    *config.Env
	client *client.Client
	auth   *auth.Service
}

// NewService はチャンネルサービスを作る。
func NewService(env *config.Env, client *client.Client) *Service {
	return &Service{
		env:    env,
		client: client,
		auth: auth.NewService(
			env.Origin,
			env.ClientID,
			env.LoadStore(),
			env.LoadSessions(),
		),
	}
}

// ResolveChannel はチャンネル指定 (UUID または "#general/a/b" 形式のパス) を ID に解決する。
func (s *Service) ResolveChannel(
	ctx context.Context,
	rt *auth.ResolvedToken,
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
	err := s.auth.Exec(
		ctx,
		rt,
		func(ctx context.Context) (*http.Response, error) {
			list, resp, apiErr := s.client.GetChannels(ctx, true)
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
func (s *Service) FetchUnreadGroups(
	ctx context.Context,
	rt *auth.ResolvedToken,
) (Groups, error) {
	if err := auth.RequireScope(rt, auth.ScopeRead); err != nil {
		return Groups{}, err
	}
	var unreads []traq.UnreadChannel
	if err := s.auth.Exec(
		ctx,
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			unreads, resp, apiErr = s.client.GetMyUnreadChannels(ctx)
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
		return s.auth.Exec(
			gctx,
			rtLocal,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				channelList, resp, apiErr = s.client.GetChannels(ctx, true)
				return resp, apiErr
			},
		)
	})
	g.Go(func() error {
		tok := *rt.Token
		rtLocal := &auth.ResolvedToken{Token: &tok, Source: rt.Source}
		return s.auth.Exec(
			gctx,
			rtLocal,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				subs, resp, apiErr = s.client.GetMyChannelSubscriptions(ctx)
				return resp, apiErr
			},
		)
	})
	g.Go(func() error {
		tok := *rt.Token
		rtLocal := &auth.ResolvedToken{Token: &tok, Source: rt.Source}
		return s.auth.Exec(
			gctx,
			rtLocal,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				starIDs, resp, apiErr = s.client.GetMyStars(ctx)
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
		if err := s.auth.Exec(
			ctx,
			rt,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				users, resp, apiErr = s.client.GetUsers(ctx)
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
	for _, sub := range subs {
		subLevels[sub.ChannelId] = sub.Level
	}
	stars := make(map[string]bool, len(starIDs))
	for _, id := range starIDs {
		stars[id] = true
	}

	groups, err := Tables{
		Paths:     pathMap,
		DM:        dmMap,
		UserNames: userNames,
		SubLevels: subLevels,
		Stars:     stars,
	}.Classify(unreads)
	if err != nil {
		return Groups{}, exit.GeneralError(err)
	}
	return groups, nil
}
