package traq

import (
	gotraq "github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/domain/auth"
	dtraq "github.com/akimon658/traq-cli/internal/domain/traq"
)

// Service は traQ への接続に必要な常駐依存を束ねる。
// Origin/ClientID/Store/Sessions は実行環境由来の長寿命設定、
// api は Origin から作られる go-traq クライアントである。
// caches は同一プロセス内の重複 API 呼び出しを抑える対象別 LRU である。
type Service struct {
	origin   string
	clientID string
	store    auth.Store
	sessions auth.SessionStore
	api      *gotraq.APIClient
	caches   *clientCaches
}

// NewService は traQ サービスを作る。
// api は Origin から1個だけ作り、使い回すこと。
func NewService(origin, clientID string, store auth.Store, sessions auth.SessionStore) *Service {
	cfg := gotraq.NewConfiguration()
	cfg.Servers = gotraq.ServerConfigurations{{URL: origin + "/api/v3"}}
	return &Service{
		origin:   origin,
		clientID: clientID,
		store:    store,
		sessions: sessions,
		api:      gotraq.NewAPIClient(cfg),
		caches:   newClientCaches(),
	}
}

// Origin は実行環境由来の origin を返す。表示用・トークン用の正典。
func (s *Service) Origin() string {
	return s.origin
}

// WithToken は解決済みトークンに束縛された認証済みクライアントを返す。
// 以降の API 呼び出しはトークン注入・スコープ検査・401時リトライを内包するため、
// 呼び出し側は ctx のみを渡せばよい。
func (s *Service) WithToken(rt *ResolvedToken) dtraq.Client {
	return &ClientImpl{svc: s, rt: rt}
}

// FinishRequest は認可コード交換に必要な要求ごとの値。
type FinishRequest struct {
	Code           string
	RedirectURI    string
	Verifier       string
	FallbackScopes []string
}
