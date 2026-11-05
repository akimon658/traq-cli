package traq

import (
	lru "github.com/hashicorp/golang-lru/v2"
	gotraq "github.com/traPtitech/go-traq"
)

// キャッシュサイズ。CLI は単発プロセスなので小さめでよい。
// ID 単位のものだけ 128 件保持し、一覧系は単一エントリで十分である。
const (
	singleEntryCacheSize = 4
	channelsCacheSize    = 2
	perIDCacheSize       = 128
)

// singleCacheKey は一覧系の単一エントリ用キー。
const singleCacheKey = "all"

// clientCaches は取得結果の対象別 LRU を束ねる。
// lru.Cache はスレッドセーフなので、Clone による並列取得でも共有できる。
type clientCaches struct {
	users     *lru.Cache[string, []gotraq.User]
	channels  *lru.Cache[bool, *gotraq.ChannelList]
	stamps    *lru.Cache[string, []gotraq.StampWithThumbnail]
	me        *lru.Cache[string, *gotraq.MyUserDetail]
	unreads   *lru.Cache[string, []gotraq.UnreadChannel]
	subs      *lru.Cache[string, []gotraq.UserSubscribeState]
	stars     *lru.Cache[string, []string]
	messages  *lru.Cache[string, *gotraq.Message]
	fileMetas *lru.Cache[string, *gotraq.FileInfo]
}

func mustNewCache[K comparable, V any](size int) *lru.Cache[K, V] {
	c, err := lru.New[K, V](size)
	if err != nil {
		panic(err)
	}
	return c
}

func newClientCaches() *clientCaches {
	return &clientCaches{
		users:     mustNewCache[string, []gotraq.User](singleEntryCacheSize),
		channels:  mustNewCache[bool, *gotraq.ChannelList](channelsCacheSize),
		stamps:    mustNewCache[string, []gotraq.StampWithThumbnail](singleEntryCacheSize),
		me:        mustNewCache[string, *gotraq.MyUserDetail](singleEntryCacheSize),
		unreads:   mustNewCache[string, []gotraq.UnreadChannel](singleEntryCacheSize),
		subs:      mustNewCache[string, []gotraq.UserSubscribeState](singleEntryCacheSize),
		stars:     mustNewCache[string, []string](singleEntryCacheSize),
		messages:  mustNewCache[string, *gotraq.Message](perIDCacheSize),
		fileMetas: mustNewCache[string, *gotraq.FileInfo](perIDCacheSize),
	}
}
