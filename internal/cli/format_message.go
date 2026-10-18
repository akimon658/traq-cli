package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/moznion/go-optional"
	"github.com/traPtitech/go-traq"
)

type messageList struct {
	label  string
	limit  int
	offset int
	order  string
	msgs   []traq.Message
	users  map[string]traq.User
	stamps map[string]string
	loc    *time.Location
	unread optional.Option[traq.UnreadChannel]
	// unreadOnly が true なら未読メッセージのみを表示するモード。
	// unread が None のときは未読0件として出力する。
	unreadOnly bool
	// totalHits が Some なら検索結果として表示する。
	totalHits optional.Option[int64]
	sort      string
	// origin はメッセージ・ファイルURL埋め込み検出用の origin。
	origin string
	// quoted は引用メッセージIDから取得済みメッセージへのマップ。取得失敗分は欠番。
	quoted map[string]traq.Message
	// files はファイルIDから取得済みメタへのマップ。取得失敗分は欠番。
	files map[string]traq.FileInfo
}

// writeMessageList はメッセージ一覧を可読性の高いテキストで出力する。
// 取得順 (order) に関わらず、表示は常に時系列 (古い順) に並べる。
func writeMessageList(w io.Writer, ml messageList) error {
	msgs := make([]traq.Message, len(ml.msgs))
	copy(msgs, ml.msgs)
	sort.SliceStable(msgs, func(i, j int) bool {
		if !msgs[i].CreatedAt.Equal(msgs[j].CreatedAt) {
			return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
		}
		return msgs[i].Id < msgs[j].Id
	})

	if total, err := ml.totalHits.Take(); err == nil {
		if _, err := fmt.Fprintf(
			w, "%s (全%d件中 %d件 limit=%d offset=%d sort=%s)\n\n",
			ml.label, total, len(msgs), ml.limit, ml.offset, ml.sort,
		); err != nil {
			return err
		}
	} else if ml.unreadOnly {
		if unread, uerr := ml.unread.Take(); uerr == nil {
			if _, err := fmt.Fprintf(
				w, "%s (未読%d件中 %d件 limit=%d offset=%d order=%s)\n\n",
				ml.label, unread.Count, len(msgs), ml.limit, ml.offset, ml.order,
			); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(w, "%s (未読0件)\n\n", ml.label); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(
		w, "%s (%d件 limit=%d offset=%d order=%s)\n\n",
		ml.label, len(msgs), ml.limit, ml.offset, ml.order,
	); err != nil {
		return err
	}

	unread, uerr := ml.unread.Take()
	divider := ""
	if uerr == nil && !ml.unreadOnly {
		divider = unreadDivider(unread)
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
		block, err := formatMessage(m, ml)
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

func unreadDivider(u traq.UnreadChannel) string {
	if u.OldestMessageId == "" {
		return ""
	}
	label := fmt.Sprintf("ここから未読 (%d件)", u.Count)
	if u.Noticeable {
		label += " [mentionあり]"
	}
	return "----- " + label + " -----"
}

func formatMessage(m traq.Message, ml messageList) (string, error) {
	header := fmt.Sprintf(
		"%s %s id:%s",
		m.CreatedAt.In(ml.loc).Format(time.RFC3339),
		userDisplay(m.UserId, ml.users),
		m.Id,
	)
	if !m.UpdatedAt.Equal(m.CreatedAt) {
		header += " [edited]"
	}
	if m.Pinned {
		header += " [pinned]"
	}

	lines := []string{header}
	clean := cleanContent(m.Content, ml.origin)
	if clean.body != "" {
		for _, line := range strings.Split(clean.body, "\n") {
			if line == "" {
				lines = append(lines, "")
			} else {
				lines = append(lines, "  "+line)
			}
		}
	}
	for _, qid := range clean.quoteIDs {
		lines = append(lines, formatQuote(qid, ml)...)
	}
	if len(clean.fileIDs) > 0 {
		lines = append(lines, "添付ファイル")
		for _, fid := range clean.fileIDs {
			lines = append(lines, "  "+formatFile(fid, ml.files))
		}
	}
	if items := formatStampItems(m.Stamps, ml.stamps); len(items) > 0 {
		lines = append(lines, "スタンプ")
		for _, it := range items {
			lines = append(lines, "  "+it)
		}
	}
	return strings.Join(lines, "\n"), nil
}

// formatQuote は引用1件分の行群を返す。取得失敗時はヘッダー1行のみ。
func formatQuote(id string, ml messageList) []string {
	qm, ok := ml.quoted[id]
	if !ok {
		return []string{fmt.Sprintf("引用 (取得失敗 id:%s)", id)}
	}
	label := userDisplay(qm.UserId, ml.users)
	header := fmt.Sprintf(
		"引用 %s %s id:%s",
		label,
		qm.CreatedAt.In(ml.loc).Format(time.RFC3339),
		qm.Id,
	)
	if !qm.UpdatedAt.Equal(qm.CreatedAt) {
		header += " [edited]"
	}
	lines := []string{header}
	if body := renderQuoteBody(qm.Content); body != "" {
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

func formatFile(id string, files map[string]traq.FileInfo) string {
	fi, ok := files[id]
	if !ok {
		return fmt.Sprintf("(取得失敗 id:%s)", id)
	}
	return fmt.Sprintf("%s (%s, %d bytes, id:%s)", fi.Name, fi.Mime, fi.Size, fi.Id)
}

func userLabel(u traq.User) string {
	label := "@" + u.Name
	if u.DisplayName != "" && u.DisplayName != u.Name {
		label += " (" + u.DisplayName + ")"
	}
	return label
}

// userDisplay はユーザーIDから表示用ラベルを返す。
// ユーザーが解決できない場合 (停止済みユーザーが一覧に無い等) は
// 実在しうる "@name" 形式と衝突しない取得失敗表示にフォールバックする。
func userDisplay(id string, users map[string]traq.User) string {
	if u, ok := users[id]; ok {
		return userLabel(u)
	}
	return userFallbackLabel(id)
}

// userFallbackLabel はユーザーIDが解決できなかったときの表示を返す。
func userFallbackLabel(id string) string {
	return fmt.Sprintf("(ユーザー解決失敗 id:%s)", id)
}

// formatStampItems はスタンプを API 返却順 (traQ-S_UI と同じく押された順) に ":name: ×count" の一覧で返す。
func formatStampItems(stamps []traq.MessageStamp, names map[string]string) []string {
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
