package message

import (
	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/domain/channel"
	"github.com/akimon658/traq-cli/internal/foundation/client"
)

// Service はメッセージ操作に必要な常駐依存を束ねる。
// 通信路・認証は構築時に固定し、要求ごとの認証情報はメソッド引数で受ける。
// Origin は auth.Service を正典とし、Service 内で重ねて保持しない。
// 詳細な責務分界は docs/architecture.md を参照すること。
type Service struct {
	client  *client.Client
	auth    *auth.Service
	channel *channel.Service
}

// NewService はメッセージサービスを作る。
func NewService(client *client.Client, auth *auth.Service) *Service {
	return &Service{
		client:  client,
		auth:    auth,
		channel: channel.NewService(client, auth),
	}
}

// origin は表示用・埋め込み検出用の origin を返す。正典は auth.Service。
func (s *Service) origin() string {
	return s.auth.Origin()
}
