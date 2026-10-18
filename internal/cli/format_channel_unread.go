package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/traPtitech/go-traq"
)

// unreadEntry は表示用に解決済みの未読1件分。
type unreadEntry struct {
	channelID string
	display   string
	count     int32
	updatedAt time.Time
}

// unreadGroups は traQ-S_UI のホームタブと同等の優先度で分類した未読。
// 順序: DM → メンション (noticeable) → 通知オン (level==notified) → お気に入り → その他。
type unreadGroups struct {
	dm       []unreadEntry
	mention  []unreadEntry
	notified []unreadEntry
	starred  []unreadEntry
	others   []unreadEntry
}

func (g unreadGroups) empty() bool {
	return len(g.dm) == 0 && len(g.mention) == 0 && len(g.notified) == 0 &&
		len(g.starred) == 0 && len(g.others) == 0
}

// buildChannelPathMap は公開チャンネル ID からフルパス ("#a/b" 形式) へのマップを作る。
func buildChannelPathMap(list *traq.ChannelList) map[string]string {
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
			cur = parent[cur]
			if cur == "" {
				break
			}
		}
		if segs == nil {
			continue
		}
		out[id] = "#" + strings.Join(segs, "/")
	}
	return out
}

// classifyUnreads は未読を DM/メンション/通知オン/お気に入り/その他に分類し、
// 各グループ内を updatedAt 降順 (同値は channelID 昇順) にソートする。
// 公開・DMいずれにも解決できない channelID はエラーを返す。
// DM相手ユーザが解決できない場合 (停止済みユーザー等) は解決失敗表示で続行する。
func classifyUnreads(
	unreads []traq.UnreadChannel,
	pathMap map[string]string,
	dmMap map[string]string,
	userNames map[string]string,
	subLevels map[string]traq.ChannelSubscribeLevel,
	stars map[string]bool,
) (unreadGroups, error) {
	var g unreadGroups
	for _, u := range unreads {
		if userID, ok := dmMap[u.ChannelId]; ok {
			name, ok := userNames[userID]
			display := userFallbackLabel(userID)
			if ok {
				display = "@" + name
			}
			g.dm = append(g.dm, unreadEntry{
				channelID: u.ChannelId,
				display:   display,
				count:     u.Count,
				updatedAt: u.UpdatedAt,
			})
			continue
		}
		path, ok := pathMap[u.ChannelId]
		if !ok {
			return unreadGroups{}, fmt.Errorf("channel not found: %s", u.ChannelId)
		}
		e := unreadEntry{
			channelID: u.ChannelId,
			display:   path,
			count:     u.Count,
			updatedAt: u.UpdatedAt,
		}
		switch {
		case u.Noticeable:
			g.mention = append(g.mention, e)
		case subLevels[u.ChannelId] == traq.CHANNELSUBSCRIBELEVEL_notified:
			g.notified = append(g.notified, e)
		case stars[u.ChannelId]:
			g.starred = append(g.starred, e)
		default:
			g.others = append(g.others, e)
		}
	}
	sortUnreadEntries(g.dm)
	sortUnreadEntries(g.mention)
	sortUnreadEntries(g.notified)
	sortUnreadEntries(g.starred)
	sortUnreadEntries(g.others)
	return g, nil
}

func sortUnreadEntries(es []unreadEntry) {
	sort.SliceStable(es, func(i, j int) bool {
		if !es[i].updatedAt.Equal(es[j].updatedAt) {
			return es[i].updatedAt.After(es[j].updatedAt)
		}
		return es[i].channelID < es[j].channelID
	})
}

// writeUnreadChannels はグループ見出し付きで未読一覧を出力する。
func writeUnreadChannels(w io.Writer, g unreadGroups) error {
	if g.empty() {
		_, err := fmt.Fprintln(w, "未読チャンネルはありません")
		return err
	}
	var b strings.Builder
	writeGroup := func(title string, es []unreadEntry) {
		if len(es) == 0 {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s (%dチャンネル)\n", title, len(es))
		for _, e := range es {
			fmt.Fprintf(&b, "%s (%d件)\n", e.display, e.count)
		}
	}
	writeGroup("DM", g.dm)
	writeGroup("メンション", g.mention)
	writeGroup("通知オン", g.notified)
	writeGroup("お気に入り", g.starred)
	writeGroup("その他", g.others)
	_, err := fmt.Fprint(w, b.String())
	return err
}
