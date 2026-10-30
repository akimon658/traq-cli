package view

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/moznion/go-optional"
	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/text"
)

// MessageList はメッセージ表示に必要な情報をまとめた DTO。
// list / search / send で共有する。コンストラクタ (NewListView 等) 経由での構築を推奨する。
type MessageList struct {
	Label  string
	Limit  int
	Offset int
	Order  string
	Msgs   []traq.Message
	Users  map[string]traq.User
	Stamps map[string]string
	Loc    *time.Location
	Unread optional.Option[traq.UnreadChannel]
	// UnreadOnly が true なら未読メッセージのみを表示するモード。
	// Unread が None のときは未読0件として出力する。
	UnreadOnly bool
	// TotalHits が Some なら検索結果として表示する。
	TotalHits optional.Option[int64]
	Sort      string
	// Origin はメッセージ・ファイルURL埋め込み検出用の origin。
	Origin string
	// Quoted は引用メッセージIDから取得済みメッセージへのマップ。取得失敗分は欠番。
	Quoted map[string]traq.Message
	// Files はファイルIDから取得済みメタへのマップ。取得失敗分は欠番。
	Files map[string]traq.FileInfo
	// SelfID と MyGroups は自分宛メンション判定用。空なら判定しない。
	SelfID   string
	MyGroups map[string]struct{}
}

// NewListView は list / unread-only 表示用の MessageList を作る。
func NewListView(
	label string,
	limit, offset int,
	order string,
	msgs []traq.Message,
	users map[string]traq.User,
	stamps map[string]string,
	loc *time.Location,
	unread optional.Option[traq.UnreadChannel],
	unreadOnly bool,
	origin string,
	quoted map[string]traq.Message,
	files map[string]traq.FileInfo,
	selfID string,
	myGroups map[string]struct{},
) MessageList {
	return MessageList{
		Label: label, Limit: limit, Offset: offset, Order: order,
		Msgs: msgs, Users: users, Stamps: stamps, Loc: loc,
		Unread: unread, UnreadOnly: unreadOnly, Origin: origin,
		Quoted: quoted, Files: files, SelfID: selfID, MyGroups: myGroups,
	}
}

// NewSearchView は search 表示用の MessageList を作る。
func NewSearchView(
	label string,
	limit, offset int,
	sort string,
	totalHits int64,
	msgs []traq.Message,
	users map[string]traq.User,
	stamps map[string]string,
	loc *time.Location,
	origin string,
	quoted map[string]traq.Message,
	files map[string]traq.FileInfo,
	selfID string,
	myGroups map[string]struct{},
) MessageList {
	return MessageList{
		Label: label, Limit: limit, Offset: offset, Sort: sort,
		TotalHits: optional.Some(totalHits),
		Msgs:      msgs, Users: users, Stamps: stamps, Loc: loc,
		Origin: origin, Quoted: quoted, Files: files,
		SelfID: selfID, MyGroups: myGroups,
	}
}

// NewSingleView は send 直後の1件表示用の MessageList を作る。
func NewSingleView(
	msg traq.Message,
	users map[string]traq.User,
	stamps map[string]string,
	loc *time.Location,
	origin string,
	quoted map[string]traq.Message,
	files map[string]traq.FileInfo,
	selfID string,
	myGroups map[string]struct{},
) MessageList {
	return MessageList{
		Msgs: []traq.Message{msg}, Users: users, Stamps: stamps, Loc: loc,
		Origin: origin, Quoted: quoted, Files: files,
		SelfID: selfID, MyGroups: myGroups,
	}
}

// WriteMessageList はメッセージ一覧を可読性の高いテキストで出力する。
// 取得順 (order) に関わらず、表示は常に時系列 (古い順) に並べる。
func WriteMessageList(w io.Writer, ml MessageList) error {
	msgs := make([]traq.Message, len(ml.Msgs))
	copy(msgs, ml.Msgs)
	sort.SliceStable(msgs, func(i, j int) bool {
		if !msgs[i].CreatedAt.Equal(msgs[j].CreatedAt) {
			return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
		}
		return msgs[i].Id < msgs[j].Id
	})

	if total, err := ml.TotalHits.Take(); err == nil {
		if _, err := fmt.Fprintf(
			w, "%s (全%d件中 %d件 limit=%d offset=%d sort=%s)\n\n",
			ml.Label, total, len(msgs), ml.Limit, ml.Offset, ml.Sort,
		); err != nil {
			return err
		}
	} else if ml.UnreadOnly {
		if unread, uerr := ml.Unread.Take(); uerr == nil {
			if _, err := fmt.Fprintf(
				w, "%s (未読%d件中 %d件 limit=%d offset=%d order=%s)\n\n",
				ml.Label, unread.Count, len(msgs), ml.Limit, ml.Offset, ml.Order,
			); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(w, "%s (未読0件)\n\n", ml.Label); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(
		w, "%s (%d件 limit=%d offset=%d order=%s)\n\n",
		ml.Label, len(msgs), ml.Limit, ml.Offset, ml.Order,
	); err != nil {
		return err
	}

	unread, uerr := ml.Unread.Take()
	divider := ""
	if uerr == nil && !ml.UnreadOnly {
		divider = UnreadDivider(unread)
	}
	dividerAt := -1
	if divider != "" {
		for i, m := range msgs {
			if m.Id == unread.OldestMessageId {
				dividerAt = i
				break
			}
		}
	}

	var blocks []string
	for i, m := range msgs {
		if i == dividerAt {
			blocks = append(blocks, divider)
		}
		block, err := FormatMessage(m, ml)
		if err != nil {
			return err
		}
		blocks = append(blocks, block)
	}
	// 未読はあるが、その先頭がこのページに含まれない場合は注記だけ出す。
	if divider != "" && dividerAt < 0 {
		blocks = append(
			[]string{fmt.Sprintf("# 未読 %d 件 (このページに未読の先頭は含まれない)", unread.Count)},
			blocks...,
		)
	}
	if len(blocks) > 0 {
		if _, err := fmt.Fprintln(w, strings.Join(blocks, "\n\n")); err != nil {
			return err
		}
	}
	return nil
}

// WriteSingleMessage は投稿直後のメッセージ1件を list/search と同じ形式で出力する。
func WriteSingleMessage(w io.Writer, ml MessageList) error {
	if len(ml.Msgs) == 0 {
		return nil
	}
	block, err := FormatMessage(ml.Msgs[0], ml)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, block)
	return err
}

// UnreadDivider は未読境界の見出しを返す。
func UnreadDivider(u traq.UnreadChannel) string {
	if u.OldestMessageId == "" {
		return ""
	}
	label := fmt.Sprintf("ここから未読 (%d件)", u.Count)
	if u.Noticeable {
		label += " [mentionあり]"
	}
	return "----- " + label + " -----"
}

// FormatMessage はメッセージ1件を表示用ブロックに整形する。
func FormatMessage(m traq.Message, ml MessageList) (string, error) {
	loc := ml.Loc
	if loc == nil {
		loc = time.Local
	}
	header := fmt.Sprintf(
		"%s %s id:%s",
		m.CreatedAt.In(loc).Format(time.RFC3339),
		UserDisplay(m.UserId, ml.Users),
		m.Id,
	)
	if !m.UpdatedAt.Equal(m.CreatedAt) {
		header += " [edited]"
	}
	if m.Pinned {
		header += " [pinned]"
	}

	clean := text.CleanContentWithHighlight(m.Content, ml.Origin, ml.SelfID, ml.MyGroups)
	if clean.SelfMentioned {
		header += " [to you]"
	}

	lines := []string{header}
	if clean.Body != "" {
		for _, line := range strings.Split(clean.Body, "\n") {
			if line == "" {
				lines = append(lines, "")
			} else {
				lines = append(lines, "  "+line)
			}
		}
	}
	for _, qid := range clean.QuoteIDs {
		lines = append(lines, FormatQuote(qid, ml)...)
	}
	if len(clean.FileIDs) > 0 {
		lines = append(lines, "添付ファイル")
		for _, fid := range clean.FileIDs {
			lines = append(lines, "  "+FormatFile(fid, ml.Files))
		}
	}
	if items := FormatStampItems(m.Stamps, ml.Stamps); len(items) > 0 {
		lines = append(lines, "スタンプ")
		for _, it := range items {
			lines = append(lines, "  "+it)
		}
	}
	return strings.Join(lines, "\n"), nil
}

// FormatQuote は引用1件分の行群を返す。取得失敗時はヘッダー1行のみ。
func FormatQuote(id string, ml MessageList) []string {
	qm, ok := ml.Quoted[id]
	if !ok {
		return []string{fmt.Sprintf("引用 (取得失敗 id:%s)", id)}
	}
	loc := ml.Loc
	if loc == nil {
		loc = time.Local
	}
	label := UserDisplay(qm.UserId, ml.Users)
	header := fmt.Sprintf(
		"引用 %s %s id:%s",
		label,
		qm.CreatedAt.In(loc).Format(time.RFC3339),
		qm.Id,
	)
	if !qm.UpdatedAt.Equal(qm.CreatedAt) {
		header += " [edited]"
	}
	lines := []string{header}
	if body := text.RenderQuoteBodyWithHighlight(
		qm.Content, ml.SelfID, ml.MyGroups,
	); body != "" {
		for _, line := range strings.Split(body, "\n") {
			if line == "" {
				lines = append(lines, "")
			} else {
				lines = append(lines, "  "+line)
			}
		}
	}
	return lines
}

// FormatFile は添付ファイル1件の表示行を返す。
func FormatFile(id string, files map[string]traq.FileInfo) string {
	fi, ok := files[id]
	if !ok {
		return fmt.Sprintf("(取得失敗 id:%s)", id)
	}
	return fmt.Sprintf("%s (%s, %d bytes, id:%s)", fi.Name, fi.Mime, fi.Size, fi.Id)
}

// FormatStampItems はスタンプを API 返却順 (traQ-S_UI と同じく押された順) に ":name: ×count" の一覧で返す。
func FormatStampItems(stamps []traq.MessageStamp, names map[string]string) []string {
	if len(stamps) == 0 {
		return nil
	}
	parts := make([]string, 0, len(stamps))
	for _, s := range stamps {
		name, ok := names[s.StampId]
		if !ok {
			name = s.StampId
		}
		parts = append(parts, fmt.Sprintf(":%s: ×%d", name, s.Count))
	}
	return parts
}
