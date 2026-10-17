package cli

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForSearchHit は word で検索し、本文に needle が現れるまでポーリングする。
// word と needle を分けるのは、Simple-Query-String-Syntax の特殊文字
// (+-|" 等) を避けるため (クエリは英数字のみの tag、本文確認は content 全体)。
func waitForSearchHit(t *testing.T, e *Env, word, needle string, args ...string) string {
	t.Helper()
	var out string
	base := append([]string{"message", "search", word}, args...)
	require.Eventually(t, func() bool {
		var err error
		out, err = runCLI(t, e, base...)
		if err != nil {
			t.Logf("search retry: %v", err)
			return false
		}
		// needle は本文行 ("  " + 本文) にしか含まれない文字列を渡すこと。
		// ES のインデックス遅延により直後は 0 件のため、本文ヒットまで待つ。
		return strings.Contains(out, "  "+needle)
	}, 90*time.Second, 2*time.Second, "search hit not found for %q, last output:\n%s", needle, out)
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

	out := waitForSearchHit(t, e, tag, content)
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

	out := waitForSearchHit(t, e, tag, content, "--in", "#general", "--from", "BOT_traq-cli")
	assert.Contains(t, out, content)
	assert.Contains(t, out, " in #general")

	// 存在しないチャンネルは exitGeneral
	_, err = runCLI(t, e, "message", "search", tag, "--in", "#does-not-exist")
	require.Error(t, err)
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitGeneral, ec.Code)

	// 存在しないユーザーは exitUsage
	_, err = runCLI(t, e, "message", "search", tag, "--from", "@no-such-user-xyz")
	require.Error(t, err)
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitUsage, ec.Code)
}
