package auth

import (
	"github.com/akimon658/traq-cli/internal/foundation/client"
)

// Service は認証に必要な常駐依存を束ねる。
// Origin/ClientID/Store/Sessions は実行環境由来の長寿命設定、
// Client は Origin から作られる通信路である。
// 要求ごとに変わる値 (コンテキスト・トークン・コード等) はメソッド引数に残す。
type Service struct {
	origin   string
	clientID string
	store    Store
	sessions SessionStore
	client   *client.Client
}

// NewService は認証サービスを作る。
func NewService(origin, clientID string, store Store, sessions SessionStore) *Service {
	return &Service{
		origin:   origin,
		clientID: clientID,
		store:    store,
		sessions: sessions,
		client:   client.New(origin),
	}
}

// FinishRequest は認可コード交換に必要な要求ごとの値。
type FinishRequest struct {
	Code           string
	RedirectURI    string
	Verifier       string
	FallbackScopes []string
}
