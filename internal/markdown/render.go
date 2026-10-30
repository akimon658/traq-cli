// Package markdown はメッセージ本文の埋め込み解決と浄化を行う純粋パーサ。
// 副作用がなく、API 呼び出しも行わない。
package markdown

import (
	"encoding/json"
	"regexp"
	"strings"
)

// urlMatchGroups はメッセージ・ファイルURL正規表現のマッチ要素数 (全体+種別+ID)。
const urlMatchGroups = 3

// minFenceTicks はフェンスとみなす最小バッククォート数。
const minFenceTicks = 3

// embedJSON は !{...} 埋め込みのペイロードに対応する。
type embedJSON struct {
	Type string `json:"type"`
	Raw  string `json:"raw"`
	ID   string `json:"id"`
}

// Cleaned は本文浄化の結果。
type Cleaned struct {
	// Body は埋め込み解決・除去後の本文 (Markdown自体は維持)。
	Body string
	// QuoteIDs は出現順・重複除去済みの引用メッセージID。
	QuoteIDs []string
	// FileIDs は出現順・重複除去済みのファイルID。
	FileIDs []string
}

var uuidPattern = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

// CollectAllEmbedIDs は複数メッセージの本文から引用・添付IDを集める。
// 一括fetch前の収集用。順序はメッセージ順・出現順、重複除去済み。
func CollectAllEmbedIDs(contents []string, origin string) (messageIDs, fileIDs []string) {
	seenMsg := map[string]bool{}
	seenFile := map[string]bool{}
	for _, c := range contents {
		cc := CleanContent(c, origin)
		for _, id := range cc.QuoteIDs {
			if !seenMsg[id] {
				seenMsg[id] = true
				messageIDs = append(messageIDs, id)
			}
		}
		for _, id := range cc.FileIDs {
			if !seenFile[id] {
				seenFile[id] = true
				fileIDs = append(fileIDs, id)
			}
		}
	}
	return messageIDs, fileIDs
}

// CleanContent は本文から埋め込みを抽出し、表示用本文を返す。
// user/channel/group のJSON埋め込みは raw に戻し、
// message/file のJSON埋め込みは本文から除去してIDだけ返す。
// 末尾のメッセージ・ファイルURL行も本文から除去してIDだけ返す
// (文中URLは本文に残しつつIDも返す = S_UIのカード表示相当)。
func CleanContent(content, origin string) Cleaned {
	lines := strings.Split(content, "\n")
	isCode := fenceCodeLines(lines)

	// 1. JSON埋め込み処理 (コード行以外)。
	var quotes, files []string
	seenQ := map[string]bool{}
	seenF := map[string]bool{}
	addQuote := func(id string) {
		if id == "" || seenQ[id] {
			return
		}
		seenQ[id] = true
		quotes = append(quotes, id)
	}
	addFile := func(id string) {
		if id == "" || seenF[id] {
			return
		}
		seenF[id] = true
		files = append(files, id)
	}
	for i, line := range lines {
		if isCode[i] {
			continue
		}
		lines[i] = mapOutsideInlineCode(line, func(s string) string {
			return replaceJSONEmbeds(s, addQuote, addFile)
		})
	}

	// 2. URL埋め込み収集 (コード行・マスク後のみ)。
	urlRe := messageFileURLRegexp(origin)
	if urlRe != nil {
		for i, line := range lines {
			if isCode[i] {
				continue
			}
			masked := maskInlineCode(line)
			for _, m := range urlRe.FindAllStringSubmatch(masked, -1) {
				if len(m) != urlMatchGroups {
					continue
				}
				switch m[1] {
				case "messages":
					addQuote(m[2])
				case "files":
					addFile(m[2])
				}
			}
		}
		// 3. 末尾の埋め込みのみ行を除去。
		lines = stripTailEmbedLines(lines, isCode, urlRe)
	}

	// 4. 末尾の空行を除去 (本文整形のため。文中空行は維持)。
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return Cleaned{Body: strings.Join(lines, "\n"), QuoteIDs: quotes, FileIDs: files}
}

// RenderQuoteBody は引用本文 (1段目) の表示用整形。
// user/channel/group は raw に戻し、入れ子の引用・添付JSONは
// プレースホルダに置換する (再帰取得はしない)。Markdownは維持。
func RenderQuoteBody(content string) string {
	lines := strings.Split(content, "\n")
	isCode := fenceCodeLines(lines)
	for i, line := range lines {
		if isCode[i] {
			continue
		}
		lines[i] = mapOutsideInlineCode(line, func(s string) string {
			return replaceJSONEmbeds(s, nil, nil)
		})
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}

// replaceJSONEmbeds は通常テキスト区間の !{...} を処理する。
// user/channel/group は raw に置き換え、message/file は除去 (空文字) して
// addQuote/addFile (nil可) にIDを通知する。
// 置換後の message/file 位置には何も残さない (節表示に一本化)。
// 引用本文用に addQuote/addFile が共に nil の場合は、
// message/file をプレースホルダ ([引用メッセージ]/[添付ファイル]) に置換する。
func replaceJSONEmbeds(s string, addQuote, addFile func(string)) string {
	nested := addQuote == nil && addFile == nil
	var b strings.Builder
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "!{")
		if start < 0 {
			b.WriteString(s[i:])
			break
		}
		start += i
		end, ok := jsonObjectEnd(s, start+1)
		if !ok {
			// 閉じ括弧が見つからない場合は残りをそのまま出す。
			b.WriteString(s[i:])
			break
		}
		var info embedJSON
		if err := json.Unmarshal([]byte(s[start+1:end]), &info); err != nil ||
			info.Type == "" || info.ID == "" {
			// JSONでない !{...} は原文維持。
			b.WriteString(s[i:end])
			i = end
			continue
		}
		b.WriteString(s[i:start])
		switch info.Type {
		case "user", "channel", "group":
			b.WriteString(info.Raw)
		case "message":
			if nested {
				b.WriteString("[引用メッセージ]")
			} else if addQuote != nil {
				addQuote(info.ID)
			}
		case "file":
			if nested {
				b.WriteString("[添付ファイル]")
			} else if addFile != nil {
				addFile(info.ID)
			}
		default:
			b.WriteString(s[start:end])
		}
		i = end
	}
	return b.String()
}

// jsonObjectEnd は s[start] == '{' であるJSONオブジェクトの終了位置 (exclusive) を返す。
// 文字列リテラルとエスケープを考慮する。見つからなければ ok=false。
func jsonObjectEnd(s string, start int) (end int, ok bool) {
	if start >= len(s) || s[start] != '{' {
		return 0, false
	}
	depth := 0
	inStr := false
	escaped := false
	for k := start; k < len(s); k++ {
		c := s[k]
		if inStr {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return k + 1, true
			}
		}
	}
	return 0, false
}

// fenceCodeLines は各行がフェンスコードブロック内か (フェンス行自体含む) を返す。
func fenceCodeLines(lines []string) []bool {
	isCode := make([]bool, len(lines))
	inFence := false
	var fence string
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if rest, ok := countBacktickRun(trimmed); ok {
			if !inFence {
				inFence = true
				fence = rest
				isCode[i] = true
				continue
			}
			// 閉じフェンスは開き以上の長さのバッククォート列。
			if len(rest) >= len(fence) && strings.Trim(rest[len(fence):], " \t") == "" {
				isCode[i] = true
				inFence = false
				fence = ""
				continue
			}
		}
		isCode[i] = inFence
	}
	return isCode
}

// countBacktickRun は行頭バッククォート列を数える。3個以上ならフェンス候補。
func countBacktickRun(s string) (string, bool) {
	n := 0
	for n < len(s) && s[n] == '`' {
		n++
	}
	if n < minFenceTicks {
		return "", false
	}
	return s[:n], true
}

// mapOutsideInlineCode は行内のインラインコード外区間にのみ fn を適用する。
func mapOutsideInlineCode(line string, fn func(string) string) string {
	var b strings.Builder
	i := 0
	for i < len(line) {
		if line[i] != '`' {
			j := i + 1
			for j < len(line) && line[j] != '`' {
				j++
			}
			b.WriteString(fn(line[i:j]))
			i = j
			continue
		}
		n := 0
		for i+n < len(line) && line[i+n] == '`' {
			n++
		}
		ticks := strings.Repeat("`", n)
		rest := line[i+n:]
		idx := strings.Index(rest, ticks)
		if idx < 0 {
			// 閉じがなければ残りはコード扱いでそのまま。
			b.WriteString(line[i:])
			break
		}
		b.WriteString(line[i : i+n+idx+n])
		i += n + idx + n
	}
	return b.String()
}

// maskInlineCode はURL検出用にインラインコード区間を空白化する。
func maskInlineCode(line string) string {
	var b strings.Builder
	i := 0
	for i < len(line) {
		if line[i] != '`' {
			j := i + 1
			for j < len(line) && line[j] != '`' {
				j++
			}
			b.WriteString(line[i:j])
			i = j
			continue
		}
		n := 0
		for i+n < len(line) && line[i+n] == '`' {
			n++
		}
		ticks := strings.Repeat("`", n)
		rest := line[i+n:]
		idx := strings.Index(rest, ticks)
		if idx < 0 {
			b.WriteString(strings.Repeat(" ", len(line)-i))
			break
		}
		b.WriteString(strings.Repeat(" ", n+idx+n))
		i += n + idx + n
	}
	return b.String()
}

// messageFileURLRegexp は origin 配下の /messages/<uuid> /files/<uuid> 検出正規表現を返す。
// origin が空なら nil。
func messageFileURLRegexp(origin string) *regexp.Regexp {
	trimmed := strings.TrimSuffix(strings.TrimSpace(origin), "/")
	if trimmed == "" {
		return nil
	}
	pattern := regexp.QuoteMeta(trimmed) + `/(messages|files)/(` + uuidPattern + `)[^\\s]*`
	return regexp.MustCompile(pattern)
}

// stripTailEmbedLines は末尾の埋め込みのみ行 (空白・URLだけの行) と
// 末尾空行を取り除く。コード行に当たったら停止する。
func stripTailEmbedLines(lines []string, isCode []bool, urlRe *regexp.Regexp) []string {
	for len(lines) > 0 {
		i := len(lines) - 1
		if isCode[i] {
			break
		}
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			lines = lines[:i]
			continue
		}
		leftover := urlRe.ReplaceAllString(maskInlineCode(lines[i]), "")
		if strings.TrimSpace(leftover) == "" {
			lines = lines[:i]
			continue
		}
		break
	}
	return lines
}
