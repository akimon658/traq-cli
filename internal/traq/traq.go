package traq

import (
	"context"
	"os"

	"github.com/traPtitech/go-traq"
)

//go:generate go tool mockgen -destination=mock_client.go -package=traq github.com/akimon658/traq-cli/internal/traq Client

// Client は認証済みクライアントの公開契約である。
// domain・ui/cli はこの型にのみ依存する。実装は ClientImpl。
// 401時リトライ等の内部状態 (svc/rt) には触れない。
type Client interface {
	Origin() string
	Clone() Client
	GetMe(ctx context.Context) (*traq.MyUserDetail, error)
	PostMessage(ctx context.Context, channelID, content string) (*traq.Message, error)
	GetChannelMessages(
		ctx context.Context,
		channelID string,
		q MessageQuery,
	) ([]traq.Message, error)
	SearchMessages(ctx context.Context, q SearchQuery) (*traq.MessageSearchResult, error)
	GetUsers(ctx context.Context) ([]traq.User, error)
	GetMyUnreadChannels(ctx context.Context) ([]traq.UnreadChannel, error)
	GetChannels(ctx context.Context, includeDm bool) (*traq.ChannelList, error)
	GetMyChannelSubscriptions(ctx context.Context) ([]traq.UserSubscribeState, error)
	GetMyStars(ctx context.Context) ([]string, error)
	GetStamps(ctx context.Context) ([]traq.StampWithThumbnail, error)
	GetMessage(ctx context.Context, messageID string) (*traq.Message, error)
	GetFileMeta(ctx context.Context, fileID string) (*traq.FileInfo, error)
	GetFile(ctx context.Context, fileID string) (*os.File, error)
	ResolveChannelID(ctx context.Context, target string) (string, error)
	GetChannelLabel(ctx context.Context, channelID string) string
	GetUserNameMap(ctx context.Context) (map[string]string, error)
	GetStampNameMap(ctx context.Context) (map[string]string, error)
	GetSelfIDAndGroups(ctx context.Context) (string, map[string]struct{}, error)
	ResolveUserIDs(ctx context.Context, inputs []string) ([]string, error)
	GetDMUserMap(ctx context.Context) (map[string]string, error)
	GetChannelPathMap(ctx context.Context) (map[string]string, error)
	GetSubscriptionLevels(
		ctx context.Context,
	) (map[string]traq.ChannelSubscribeLevel, error)
	GetStarSet(ctx context.Context) (map[string]bool, error)
}

var _ Client = (*ClientImpl)(nil)
