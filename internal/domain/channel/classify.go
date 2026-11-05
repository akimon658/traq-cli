package channel

import (
	"fmt"
	"sort"
	"time"

	gotraq "github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/traq"
)

// Entry は表示用に解決済みの未読1件分。
type Entry struct {
	ChannelID string
	Display   string
	Count     int32
	UpdatedAt time.Time
}

// Groups は traQ-S_UI のホームタブと同等の優先度で分類した未読。
// 順序: DM → メンション (noticeable) → 通知オン (level==notified) → お気に入り → その他。
type Groups struct {
	DM       []Entry
	Mention  []Entry
	Notified []Entry
	Starred  []Entry
	Others   []Entry
}

// Empty は全グループが空かどうかを返す。
func (g Groups) Empty() bool {
	return len(g.DM) == 0 && len(g.Mention) == 0 && len(g.Notified) == 0 &&
		len(g.Starred) == 0 && len(g.Others) == 0
}

// Tables は未読分類に必要な参照データを束ねる。
// 解決済みマップ群をレシーバに保持し、要求ごとの未読一覧を引数で受ける。
type Tables struct {
	Paths     map[string]string
	DM        map[string]string
	UserNames map[string]string
	SubLevels map[string]gotraq.ChannelSubscribeLevel
	Stars     map[string]bool
}

// Classify は未読を DM/メンション/通知オン/お気に入り/その他に分類し、
// 各グループ内を updatedAt 降順 (同値は channelID 昇順) にソートする。
// 公開・DMいずれにも解決できない channelID はエラーを返す。
// DM相手ユーザが解決できない場合 (停止済みユーザー等) は解決失敗表示で続行する。
func (t Tables) Classify(
	unreads []gotraq.UnreadChannel,
) (Groups, error) {
	var g Groups
	for _, u := range unreads {
		if userID, ok := t.DM[u.ChannelId]; ok {
			name, ok := t.UserNames[userID]
			display := traq.FormatUserFallback(userID)
			if ok {
				display = "@" + name
			}
			g.DM = append(g.DM, Entry{
				ChannelID: u.ChannelId,
				Display:   display,
				Count:     u.Count,
				UpdatedAt: u.UpdatedAt,
			})
			continue
		}
		path, ok := t.Paths[u.ChannelId]
		if !ok {
			return Groups{}, fmt.Errorf("channel not found: %s", u.ChannelId)
		}
		e := Entry{
			ChannelID: u.ChannelId,
			Display:   path,
			Count:     u.Count,
			UpdatedAt: u.UpdatedAt,
		}
		switch {
		case u.Noticeable:
			g.Mention = append(g.Mention, e)
		case t.SubLevels[u.ChannelId] == gotraq.CHANNELSUBSCRIBELEVEL_notified:
			g.Notified = append(g.Notified, e)
		case t.Stars[u.ChannelId]:
			g.Starred = append(g.Starred, e)
		default:
			g.Others = append(g.Others, e)
		}
	}
	sortEntries(g.DM)
	sortEntries(g.Mention)
	sortEntries(g.Notified)
	sortEntries(g.Starred)
	sortEntries(g.Others)
	return g, nil
}

func sortEntries(es []Entry) {
	sort.SliceStable(es, func(i, j int) bool {
		if !es[i].UpdatedAt.Equal(es[j].UpdatedAt) {
			return es[i].UpdatedAt.After(es[j].UpdatedAt)
		}
		return es[i].ChannelID < es[j].ChannelID
	})
}
