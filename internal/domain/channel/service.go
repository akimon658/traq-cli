package channel

import (
	"context"

	gotraq "github.com/traPtitech/go-traq"
	"golang.org/x/sync/errgroup"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
)

// Service はチャンネル操作に必要な常駐依存を束ねる。
// 認証済みクライアントを構築時に固定し、要求ごとの値はメソッド引数で受ける。
type Service struct {
	api traq.Client
}

// NewService はチャンネルサービスを作る。
func NewService(api traq.Client) *Service {
	return &Service{
		api: api,
	}
}

// GetUnreadGroups は未読情報を取得し、表示用のグループに分類して返す。
func (s *Service) GetUnreadGroups(
	ctx context.Context,
) (Groups, error) {
	unreads, err := s.api.GetMyUnreadChannels(ctx)
	if err != nil {
		return Groups{}, err
	}
	if len(unreads) == 0 {
		return Groups{}, nil
	}

	// 最初の取得でトークンは検証/更新済みのため、残りの独立した取得は並列化する。
	// 各 goroutine はトークンのコピーを参照し、リトライ時の書き戻しのデータ競合だけを避ける。
	// store は共有のため、並列中の同時401で同一 refresh token が複数回使われると2回目以降が失敗し得るが、最初の取得で検証済みのため可能性は低い。
	// 取得結果の整形は traq 側のヘルパーが行い、ここでは引き回さない。
	var (
		pathMap   map[string]string
		dmMap     map[string]string
		userNames map[string]string
		subLevels map[string]gotraq.ChannelSubscribeLevel
		stars     map[string]bool
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		pathMap, err = s.api.Clone().GetChannelPathMap(gctx)
		return err
	})
	g.Go(func() error {
		var err error
		dmMap, err = s.api.Clone().GetDMUserMap(gctx)
		return err
	})
	g.Go(func() error {
		var err error
		subLevels, err = s.api.Clone().GetSubscriptionLevels(gctx)
		return err
	})
	g.Go(func() error {
		var err error
		stars, err = s.api.Clone().GetStarSet(gctx)
		return err
	})
	if err := g.Wait(); err != nil {
		return Groups{}, err
	}

	for _, u := range unreads {
		if _, ok := dmMap[u.ChannelId]; ok {
			names, err := s.api.GetUserNameMap(ctx)
			if err != nil {
				return Groups{}, err
			}
			userNames = names
			break
		}
	}
	if userNames == nil {
		userNames = map[string]string{}
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
