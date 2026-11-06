package traq

import (
	"fmt"
	"strings"
	"uuid"
)

// FormatUserFallback はユーザーIDが解決できなかったときの表示を返す。
func FormatUserFallback(id string) string {
	return fmt.Sprintf("(ユーザー解決失敗 id:%s)", id)
}

// DisplayLabel は表示用のチャンネル名を組み立てる。UUID 指定ならそのまま、
// パス指定なら先頭に "#" を付ける。
func DisplayLabel(target string) string {
	if _, err := uuid.Parse(target); err == nil {
		return target
	}
	return "#" + strings.TrimPrefix(target, "#")
}
