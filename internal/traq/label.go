package traq

import (
	"fmt"
	"strings"
	"uuid"

	gotraq "github.com/traPtitech/go-traq"
)

// FormatUserFallback はユーザーIDが解決できなかったときの表示を返す。
// 実在しうる "@name" 形式と衝突しない形式にする。正典はこの関数である。
func FormatUserFallback(id string) string {
	return fmt.Sprintf("(ユーザー解決失敗 id:%s)", id)
}

// fullPathMap は公開チャンネル ID から素パス ("a/b" 形式、# なし) へのマップを作る。
func fullPathMap(list *gotraq.ChannelList) map[string]string {
	byID := make(map[string]gotraq.Channel, len(list.Public))
	parent := make(map[string]string, len(list.Public))
	for _, ch := range list.Public {
		byID[ch.Id] = ch
		parent[ch.Id] = ""
		if p := ch.ParentId.Get(); p != nil {
			parent[ch.Id] = *p
		}
	}
	out := make(map[string]string, len(list.Public))
	for id := range byID {
		var segs []string
		cur := id
		for i := 0; i <= len(byID); i++ {
			ch, ok := byID[cur]
			if !ok {
				segs = nil
				break
			}
			segs = append([]string{ch.Name}, segs...)
			next := parent[cur]
			if next == "" || next == id {
				break
			}
			cur = next
		}
		if segs == nil {
			continue
		}
		out[id] = strings.Join(segs, "/")
	}
	return out
}

// BuildPathMap は公開チャンネル ID からフルパス ("#a/b" 形式) へのマップを作る。
func BuildPathMap(list *gotraq.ChannelList) map[string]string {
	full := fullPathMap(list)
	out := make(map[string]string, len(full))
	for id, p := range full {
		out[id] = "#" + p
	}
	return out
}

// FindByPath は素パス ("a/b" 形式、# の有無は問わない) からチャンネル ID を探す。
func FindByPath(list *gotraq.ChannelList, path string) (string, error) {
	path = strings.TrimPrefix(path, "#")
	for id, p := range fullPathMap(list) {
		if p == path {
			return id, nil
		}
	}
	return "", fmt.Errorf("channel not found: %s", path)
}

// FormatChannelLabel はチャンネルIDから表示用ラベルを返す。
// 公開チャンネルは "#a/b"、DM は "@name" (ユーザー不明時は FormatUserFallback)。
// どちらにも解決できないときは channelID をそのまま返す (UUID フォールバック)。
func FormatChannelLabel(
	list *gotraq.ChannelList,
	userNames map[string]string,
	channelID string,
) string {
	if list != nil {
		if p, ok := BuildPathMap(list)[channelID]; ok {
			return p
		}
		for _, dm := range list.Dm {
			if dm.Id != channelID {
				continue
			}
			if name, ok := userNames[dm.UserId]; ok {
				return "@" + name
			}
			return FormatUserFallback(dm.UserId)
		}
	}
	return channelID
}

// DisplayLabel は表示用のチャンネル名を組み立てる。UUID 指定ならそのまま、
// パス指定なら先頭に "#" を付ける。
func DisplayLabel(target string) string {
	if _, err := uuid.Parse(target); err == nil {
		return target
	}
	return "#" + strings.TrimPrefix(target, "#")
}
