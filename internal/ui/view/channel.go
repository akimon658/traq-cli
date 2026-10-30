package view

import (
	"fmt"
	"io"
	"strings"

	"github.com/akimon658/traq-cli/internal/domain/channel"
)

// WriteUnreadChannels はグループ見出し付きで未読一覧を出力する。
func WriteUnreadChannels(w io.Writer, g channel.Groups) error {
	if g.Empty() {
		_, err := fmt.Fprintln(w, "未読チャンネルはありません")
		return err
	}
	var b strings.Builder
	writeGroup := func(title string, es []channel.Entry) {
		if len(es) == 0 {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s (%dチャンネル)\n", title, len(es))
		for _, e := range es {
			fmt.Fprintf(&b, "%s (%d件)\n", e.Display, e.Count)
		}
	}
	writeGroup("DM", g.DM)
	writeGroup("メンション", g.Mention)
	writeGroup("通知オン", g.Notified)
	writeGroup("お気に入り", g.Starred)
	writeGroup("その他", g.Others)
	_, err := fmt.Fprint(w, b.String())
	return err
}
