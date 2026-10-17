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
	// totalHits が Some なら検索結果として表示する。
	totalHits optional.Option[int64]
	sort      string
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
	} else if _, err := fmt.Fprintf(
		w, "%s (%d件 limit=%d offset=%d order=%s)\n\n",
		ml.label, len(msgs), ml.limit, ml.offset, ml.order,
	); err != nil {
		return err
	}

	unread, uerr := ml.unread.Take()
	divider := ""
	if uerr == nil {
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
	user, ok := ml.users[m.UserId]
	if !ok {
		return "", fmt.Errorf("failed to resolve the user of message %s: %s", m.Id, m.UserId)
	}

	header := fmt.Sprintf(
		"%s %s id:%s",
		m.CreatedAt.In(ml.loc).Format(time.RFC3339),
		userLabel(user),
		m.Id,
	)
	if !m.UpdatedAt.Equal(m.CreatedAt) {
		header += " [edited]"
	}
	if m.Pinned {
		header += " [pinned]"
	}

	lines := []string{header}
	if m.Content != "" {
		for _, line := range strings.Split(m.Content, "\n") {
			lines = append(lines, "  "+line)
		}
	}
	if s := formatStamps(m.Stamps, ml.stamps); s != "" {
		lines = append(lines, s)
	}
	return strings.Join(lines, "\n"), nil
}

func userLabel(u traq.User) string {
	label := "@" + u.Name
	if u.DisplayName != "" && u.DisplayName != u.Name {
		label += " (" + u.DisplayName + ")"
	}
	return label
}

// formatStamps はスタンプを本文と区別できるようインデントせずに 1 行で返す。
func formatStamps(stamps []traq.MessageStamp, names map[string]string) string {
	if len(stamps) == 0 {
		return ""
	}
	parts := make([]string, 0, len(stamps))
	for _, s := range stamps {
		name, ok := names[s.StampId]
		if !ok {
			name = s.StampId
		}
		parts = append(parts, fmt.Sprintf(":%s:%d", name, s.Count))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}
