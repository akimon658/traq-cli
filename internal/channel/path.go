package channel

import (
	"fmt"
	"strings"
	"uuid"

	"github.com/traPtitech/go-traq"
)

// fullPathMap は公開チャンネル ID から素パス ("a/b" 形式、# なし) へのマップを作る。
func fullPathMap(list *traq.ChannelList) map[string]string {
	byID := make(map[string]traq.Channel, len(list.Public))
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
func BuildPathMap(list *traq.ChannelList) map[string]string {
	full := fullPathMap(list)
	out := make(map[string]string, len(full))
	for id, p := range full {
		out[id] = "#" + p
	}
	return out
}

// FindByPath は素パス ("a/b" 形式、# の有無は問わない) からチャンネル ID を探す。
func FindByPath(list *traq.ChannelList, path string) (string, error) {
	path = strings.TrimPrefix(path, "#")
	for id, p := range fullPathMap(list) {
		if p == path {
			return id, nil
		}
	}
	return "", fmt.Errorf("channel not found: %s", path)
}

// DisplayLabel は表示用のチャンネル名を組み立てる。UUID 指定ならそのまま、
// パス指定なら先頭に "#" を付ける。
func DisplayLabel(target string) string {
	if _, err := uuid.Parse(target); err == nil {
		return target
	}
	return "#" + strings.TrimPrefix(target, "#")
}
