package traq

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/moznion/go-optional"
	"github.com/traPtitech/go-traq"
)

// TokenContext は ctx に Bearer トークンを埋め込む。go-traq が Authorization ヘッダを付ける。
// 直接使うのは traq パッケージ内部とテストに限ること。通常は ClientImpl メソッドを使う。
func TokenContext(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, traq.ContextAccessToken, token)
}

// APIError は traQ API のエラー応答を保持する。
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("traQ API error (HTTP %d): %s", e.Status, e.Body)
	}
	return fmt.Sprintf("traQ API error (HTTP %d)", e.Status)
}

// WrapError は go-traq のエラーを APIError に整形する。
func WrapError(err error, resp *http.Response) error {
	if err == nil {
		return nil
	}
	if resp == nil {
		return err
	}
	var ge *traq.GenericOpenAPIError
	if errors.As(err, &ge) {
		return &APIError{Status: resp.StatusCode, Body: strings.TrimRight(string(ge.Body()), "\n")}
	}
	return err
}

// MessageQuery はチャンネルメッセージ取得のパラメータ。
type MessageQuery struct {
	Limit     int32
	Offset    int32
	Since     optional.Option[time.Time]
	Until     optional.Option[time.Time]
	Inclusive bool
	Order     string
}

// SearchQuery はメッセージ検索 (GET /messages) のパラメータ。
type SearchQuery struct {
	Word           optional.Option[string]
	After          optional.Option[time.Time]
	Before         optional.Option[time.Time]
	In             optional.Option[string]
	To             optional.Option[[]string]
	From           optional.Option[[]string]
	Citation       optional.Option[string]
	Bot            optional.Option[bool]
	HasURL         optional.Option[bool]
	HasAttachments optional.Option[bool]
	HasImage       optional.Option[bool]
	HasVideo       optional.Option[bool]
	HasAudio       optional.Option[bool]
	Limit          int32
	Offset         int32
	Sort           string
}

// 以下は生の API 呼び出しで、ctx には TokenContext でトークンが埋め込まれている前提。
// トークン解決・スコープ検査・リトライは ClientImpl が行うため、直接呼ばないこと。

func (s *Service) rawGetMe(
	ctx context.Context,
) (*traq.MyUserDetail, *http.Response, error) {
	return s.api.MeAPI.GetMe(ctx).Execute()
}

// rawPostMessage はチャンネルにメッセージを投稿する。
func (s *Service) rawPostMessage(
	ctx context.Context,
	channelID, content string,
) (*traq.Message, *http.Response, error) {
	return s.api.MessageAPI.
		PostMessage(ctx, channelID).
		PostMessageRequest(traq.PostMessageRequest{Content: content}).
		Execute()
}

// rawGetChannelMessages はチャンネルのメッセージを取得する。
func (s *Service) rawGetChannelMessages(
	ctx context.Context,
	channelID string,
	q MessageQuery,
) ([]traq.Message, *http.Response, error) {
	req := s.api.MessageAPI.GetMessages(ctx, channelID).
		Limit(q.Limit).
		Offset(q.Offset).
		Inclusive(q.Inclusive).
		Order(q.Order)
	q.Since.IfSome(func(v time.Time) {
		req = req.Since(v)
	})
	q.Until.IfSome(func(v time.Time) {
		req = req.Until(v)
	})
	return req.Execute()
}

// rawSearchMessages はメッセージを検索する。
func (s *Service) rawSearchMessages(
	ctx context.Context,
	q SearchQuery,
) (*traq.MessageSearchResult, *http.Response, error) {
	req := s.api.MessageAPI.SearchMessages(ctx).
		Limit(q.Limit).
		Offset(q.Offset).
		Sort(q.Sort)
	q.Word.IfSome(func(v string) {
		req = req.Word(v)
	})
	q.After.IfSome(func(v time.Time) {
		req = req.After(v)
	})
	q.Before.IfSome(func(v time.Time) {
		req = req.Before(v)
	})
	q.In.IfSome(func(v string) {
		req = req.In(v)
	})
	q.To.IfSome(func(v []string) {
		req = req.To(v)
	})
	q.From.IfSome(func(v []string) {
		req = req.From(v)
	})
	q.Citation.IfSome(func(v string) {
		req = req.Citation(v)
	})
	q.Bot.IfSome(func(v bool) {
		req = req.Bot(v)
	})
	q.HasURL.IfSome(func(v bool) {
		req = req.HasURL(v)
	})
	q.HasAttachments.IfSome(func(v bool) {
		req = req.HasAttachments(v)
	})
	q.HasImage.IfSome(func(v bool) {
		req = req.HasImage(v)
	})
	q.HasVideo.IfSome(func(v bool) {
		req = req.HasVideo(v)
	})
	q.HasAudio.IfSome(func(v bool) {
		req = req.HasAudio(v)
	})
	return req.Execute()
}

// rawGetUsers は停止済み・無効化済みユーザーを含む全ユーザーを取得する。
// include-suspended を指定しないと有効なユーザーしか返らず、
// 停止済みユーザーの投稿を表示する際に名前解決へ失敗するため常に指定する。
func (s *Service) rawGetUsers(ctx context.Context) ([]traq.User, *http.Response, error) {
	return s.api.UserAPI.GetUsers(ctx).IncludeSuspended(true).Execute()
}

func (s *Service) rawGetMyUnreadChannels(
	ctx context.Context,
) ([]traq.UnreadChannel, *http.Response, error) {
	return s.api.MeAPI.GetMyUnreadChannels(ctx).Execute()
}

func (s *Service) rawGetChannels(
	ctx context.Context, includeDm bool,
) (*traq.ChannelList, *http.Response, error) {
	return s.api.ChannelAPI.GetChannels(ctx).IncludeDm(includeDm).Execute()
}

func (s *Service) rawGetMyChannelSubscriptions(
	ctx context.Context,
) ([]traq.UserSubscribeState, *http.Response, error) {
	return s.api.MeAPI.GetMyChannelSubscriptions(ctx).Execute()
}

func (s *Service) rawGetMyStars(
	ctx context.Context,
) ([]string, *http.Response, error) {
	return s.api.MeAPI.GetMyStars(ctx).Execute()
}

func (s *Service) rawGetStamps(
	ctx context.Context,
) ([]traq.StampWithThumbnail, *http.Response, error) {
	return s.api.StampAPI.GetStamps(ctx).Execute()
}

func (s *Service) rawGetMessage(
	ctx context.Context, messageID string,
) (*traq.Message, *http.Response, error) {
	return s.api.MessageAPI.GetMessage(ctx, messageID).Execute()
}

func (s *Service) rawGetFileMeta(
	ctx context.Context, fileID string,
) (*traq.FileInfo, *http.Response, error) {
	return s.api.FileAPI.GetFileMeta(ctx, fileID).Execute()
}

// rawGetFile はファイル本体を取得する。go-traq はレスポンス全体をメモリに読み込んだ上で
// 一時ファイルに書き出すため、返り値のファイルは呼び出し側が Close と Remove を行うこと。
func (s *Service) rawGetFile(
	ctx context.Context, fileID string,
) (*os.File, *http.Response, error) {
	return s.api.FileAPI.GetFile(ctx, fileID).Execute()
}
