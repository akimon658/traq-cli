package message

import (
	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/domain/channel"
	"github.com/akimon658/traq-cli/internal/foundation/client"
)

// Service はメッセージ操作に必要な常駐依存を束ねる。
// Origin・通信路・認証は構築時に固定し、要求ごとの認証情報はメソッド引数で受ける。
// 詳細な責務分界は docs/architecture.md を参照すること。
type Service struct {
	origin  string
	client  *client.Client
	auth    *auth.Service
	channel *channel.Service
}

// NewService はメッセージサービスを作る。
func NewService(origin string, client *client.Client, auth *auth.Service) *Service {
	return &Service{
		origin:  origin,
		client:  client,
		auth:    auth,
		channel: channel.NewService(client, auth),
	}
}
