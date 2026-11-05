package message

import (
	"github.com/akimon658/traq-cli/internal/traq"
)

// Service はメッセージ操作に必要な常駐依存を束ねる。
// 認証済みクライアントを構築時に固定し、要求ごとの値はメソッド引数で受ける。
// Origin は traq.Client を正典とし、Service 内で重ねて保持しない。
// 詳細な責務分界は docs/architecture.md を参照すること。
type Service struct {
	api traq.Client
}

// NewService はメッセージサービスを作る。
func NewService(api traq.Client) *Service {
	return &Service{
		api: api,
	}
}
