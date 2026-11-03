package message

import (
	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/domain/channel"
	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/config"
)

// Service はメッセージ操作に必要な常駐依存を束ねる。
// 実行環境と通信路は構築時に固定し、要求ごとの認証情報はメソッド引数で受ける。
type Service struct {
	env     *config.Env
	client  *client.Client
	auth    *auth.Service
	channel *channel.Service
}

// NewService はメッセージサービスを作る。
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
		channel: channel.NewService(env, client),
	}
}
