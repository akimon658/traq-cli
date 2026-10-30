package message

import (
	"time"

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

// Enrichment は表示に必要な付随情報。引用・添付の解決失敗分は欠番になる。
type Enrichment struct {
	Users  map[string]traq.User
	Stamps map[string]string
	Quoted map[string]traq.Message
	Files  map[string]traq.FileInfo
	// SelfID と MyGroups は自分宛メンション判定用。取得できなかった場合は空。
	SelfID   string
	MyGroups map[string]struct{}
}

// ListResult は message list の結果。
type ListResult struct {
	Label  string
	Limit  int
	Offset int
	Order  string
	Msgs   []traq.Message
	Enrichment
	Loc        *time.Location
	Unread     optional.Option[traq.UnreadChannel]
	UnreadOnly bool
	Origin     string
}

// SearchResult は message search の結果。
type SearchResult struct {
	Label     string
	Limit     int
	Offset    int
	Sort      string
	TotalHits int64
	Msgs      []traq.Message
	Enrichment
	Loc    *time.Location
	Origin string
}

// SendResult は message send の結果 (投稿直後の1件)。
type SendResult struct {
	Msg traq.Message
	Enrichment
	Loc    *time.Location
	Origin string
}
