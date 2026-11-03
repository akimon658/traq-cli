package cli

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// searchWait は検索ヒット待ちに必要な常駐依存を束ねる。
type searchWait struct {
	env    *config.Env
	word   string
	needle string
	args   []string
}

// wait は word で検索し、本文に needle が現れるまでポーリングする。
// word と needle を分けるのは、Simple-Query-String-Syntax の特殊文字
// (+-|" 等) を避けるため (クエリは英数字のみの tag、本文確認は content 全体)。
func (w searchWait) wait(t *testing.T) string {
	t.Helper()
	var out string
	base := append([]string{"message", "search", w.word}, w.args...)
	require.Eventually(t, func() bool {
		var err error
		out, err = runCLI(t, w.env, base...)
		if err != nil {
			t.Logf("search retry: %v", err)
			return false
		}
		// needle は本文行 ("  " + 本文) にしか含まれない文字列を渡すこと。
		// ES のインデックス遅延により直後は 0 件のため、本文ヒットまで待つ。
		return strings.Contains(out, "  "+w.needle)
	}, 90*time.Second, 2*time.Second, "search hit not found for %q, last output:\n%s", w.needle, out)
	return out
}

func TestMessageSearch(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	e := botEnv(t, testTraqToken)
	tag := strings.ReplaceAll(uuid.New().String(), "-", "")
	content := "search-" + tag
	_, err := runCLI(t, e, "message", "send", "#general", content)
	require.NoError(t, err, "message send failed")

	out := searchWait{env: e, word: tag, needle: content}.wait(t)
	assert.Contains(t, out, "search \""+tag+"\"")
	assert.Contains(t, out, "limit=10")
	assert.Contains(t, out, "sort=-createdAt")
	assert.Contains(t, out, "@BOT_traq-cli", "the author name should be resolved")
	assert.NotContains(t, out, `"id"`, "output should be human-readable text, not JSON")
}

func TestMessageSearchInAndFrom(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	e := botEnv(t, testTraqToken)
	tag := strings.ReplaceAll(uuid.New().String(), "-", "")
	content := "searchfilter-" + tag
	_, err := runCLI(t, e, "message", "send", "#general", content)
	require.NoError(t, err, "message send failed")

	out := searchWait{
		env:    e,
		word:   tag,
		needle: content,
		args:   []string{"--in", "#general", "--from", "BOT_traq-cli"},
	}.wait(t)
	assert.Contains(t, out, content)
	assert.Contains(t, out, " in #general")

	// 存在しないチャンネルは exit.General
	_, err = runCLI(t, e, "message", "search", tag, "--in", "#does-not-exist")
	require.Error(t, err)
	var ec *exit.Error
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.General, ec.Code)

	// 存在しないユーザーは exit.Usage
	_, err = runCLI(t, e, "message", "search", tag, "--from", "@no-such-user-xyz")
	require.Error(t, err)
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.Usage, ec.Code)
}
