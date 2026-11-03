package channel

import (
	"context"
	"strings"
	"uuid"

	gotraq "github.com/traPtitech/go-traq"
	"golang.org/x/sync/errgroup"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
)

// Service はチャンネル操作に必要な常駐依存を束ねる。
// 認証済みクライアントを構築時に固定し、要求ごとの値はメソッド引数で受ける。
type Service struct {
	api *traq.Client
}

// NewService はチャンネルサービスを作る。
func NewService(api *traq.Client) *Service {
	return &Service{
		api: api,
	}
}

// ResolveChannel はチャンネル指定 (UUID または "#general/a/b" 形式のパス) を ID に解決する。
func (s *Service) ResolveChannel(
	ctx context.Context,
	target string,
) (string, error) {
	if _, err := uuid.Parse(target); err == nil {
		return target, nil
	}
	path := strings.TrimPrefix(target, "#")
	list, err := s.api.GetChannels(ctx, true)
	if err != nil {
		return "", err
	}
	channelID, err := FindByPath(list, path)
	if err != nil {
		return "", exit.GeneralError(err)
	}
	if channelID == "" {
		return "", exit.Generalf("channel not found: %s", target)
	}
	return channelID, nil
}

// FetchUnreadGroups は未読情報を取得し、表示用のグループに分類して返す。
func (s *Service) FetchUnreadGroups(
	ctx context.Context,
) (Groups, error) {
	unreads, err := s.api.GetMyUnreadChannels(ctx)
	if err != nil {
		return Groups{}, err
	}
	if len(unreads) == 0 {
		return Groups{}, nil
	}

	// 最初の取得でトークンは検証/更新済みのため、残りの独立した3取得は並列化する。
	// 各 goroutine はトークンのコピーを参照し、リトライ時の書き戻しのデータ競合だけを避ける。
	// store は共有のため、並列中の同時401で同一 refresh token が複数回使われると2回目以降が失敗し得るが、最初の取得で検証済みのため可能性は低い。
	var channelList *gotraq.ChannelList
	var subs []gotraq.UserSubscribeState
	var starIDs []string
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		channelList, err = s.api.Clone().GetChannels(gctx, true)
		return err
	})
	g.Go(func() error {
		var err error
		subs, err = s.api.Clone().GetMyChannelSubscriptions(gctx)
		return err
	})
	g.Go(func() error {
		var err error
		starIDs, err = s.api.Clone().GetMyStars(gctx)
		return err
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
		users, err := s.api.GetUsers(ctx)
		if err != nil {
			return Groups{}, err
		}
		for _, u := range users {
			userNames[u.Id] = u.Name
		}
	}

	subLevels := make(map[string]gotraq.ChannelSubscribeLevel, len(subs))
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
