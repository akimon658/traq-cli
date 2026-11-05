package message

import (
	"github.com/moznion/go-optional"
	"github.com/traPtitech/go-traq"
)

// ListOptions は message list の入力。フラグ値をそのまま渡す。
type ListOptions struct {
	Limit      int
	Offset     int
	Since      string
	Until      string
	Inclusive  bool
	Order      string
	ShowUnread bool
	UnreadOnly bool
}

// SearchOptions は message search の入力。フラグ値をそのまま渡す。
type SearchOptions struct {
	In             string
	From           []string
	To             []string
	After          string
	Before         string
	Citation       string
	Bot            optional.Option[bool]
	HasURL         optional.Option[bool]
	HasAttachments optional.Option[bool]
	HasImage       optional.Option[bool]
	HasVideo       optional.Option[bool]
	HasAudio       optional.Option[bool]
	Limit          int
	Offset         int
	Sort           string
}

// ListResult は message list の結果。表示に必要な解決は view が traq.Client 経由で行う。
type ListResult struct {
	Label  string
	Limit  int
	Offset int
	Order  string
	Msgs   []traq.Message
	Unread optional.Option[traq.UnreadChannel]
	// UnreadOnly が true なら未読メッセージのみを表示するモード。
	// Unread が None のときは未読0件として出力する。
	UnreadOnly bool
}

// SearchResult は message search の結果。
type SearchResult struct {
	Label     string
	Limit     int
	Offset    int
	Sort      string
	TotalHits int64
	Msgs      []traq.Message
}

// SendResult は message send の結果 (投稿直後の1件)。
type SendResult struct {
	Msg traq.Message
}

// GetResult は message get の結果 (1件)。
type GetResult struct {
	Msg traq.Message
	// ChannelLabel は投稿先チャンネルの表示用ラベル ("#a/b" / DM は "@name")。
	// 解決できないときはチャンネル UUID。
	ChannelLabel string
}
