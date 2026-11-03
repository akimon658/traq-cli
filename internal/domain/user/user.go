package user

import (
	"context"
	"fmt"

	gotraq "github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/traq"
)

// FallbackLabel はユーザーIDが解決できなかったときの表示を返す。
// channel と view で共有する正規の形式。
func FallbackLabel(id string) string {
	return fmt.Sprintf("(ユーザー解決失敗 id:%s)", id)
}

// Service はユーザー操作に必要な常駐依存を束ねる。
// 認証済みクライアントを構築時に固定し、要求ごとの値はメソッド引数で受ける。
type Service struct {
	api *traq.Client
}

// NewService はユーザーサービスを作る。
func NewService(api *traq.Client) *Service {
	return &Service{
		api: api,
	}
}

// FetchMe は自分のユーザー詳細を取得する。
func (s *Service) FetchMe(
	ctx context.Context,
) (*gotraq.MyUserDetail, error) {
	return s.api.GetMe(ctx)
}
