package text

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testQuoteID1 = "11111111-1111-1111-1111-111111111111"
	testQuoteID2 = "22222222-2222-2222-2222-222222222222"
	testFileID1  = "33333333-3333-3333-3333-333333333333"
	testOrigin   = "https://q.trap.jp"
)

func TestCleanContentMentionJSON(t *testing.T) {
	t.Parallel()
	in := `hello !{"type":"user","raw":"@alice","id":"u1"} and !{"type":"channel","raw":"#general","id":"c1"} and !{"type":"group","raw":"@team","id":"g1"}`
	got := CleanContent(in, "")
	assert.Equal(t, "hello @alice and #general and @team", got.Body)
	assert.Empty(t, got.QuoteIDs)
	assert.Empty(t, got.FileIDs)
}

func TestCleanContentMessageFileJSON(t *testing.T) {
	t.Parallel()
	in := "see this\n" +
		`!{"type":"message","raw":"引用","id":"` + testQuoteID1 + `"}` + "\n" +
		`file !{"type":"file","raw":"a.png","id":"` + testFileID1 + `"} end`
	got := CleanContent(in, "")
	assert.Equal(t, "see this\n\nfile  end", got.Body)
	require.Equal(t, []string{testQuoteID1}, got.QuoteIDs)
	require.Equal(t, []string{testFileID1}, got.FileIDs)
}

func TestCleanContentJSONDedupOrder(t *testing.T) {
	t.Parallel()
	in := `!{"type":"message","raw":"x","id":"` + testQuoteID1 + `"} ` +
		`!{"type":"message","raw":"x","id":"` + testQuoteID2 + `"} ` +
		`!{"type":"message","raw":"x","id":"` + testQuoteID1 + `"}`
	got := CleanContent(in, "")
	assert.Equal(t, []string{testQuoteID1, testQuoteID2}, got.QuoteIDs)
}

func TestCleanContentBrokenJSONKept(t *testing.T) {
	t.Parallel()
	in := `!{"type":"user","raw":"@alice"`
	got := CleanContent(in, "")
	assert.Equal(t, in, got.Body)
	assert.Empty(t, got.QuoteIDs)
}

func TestCleanContentUnknownTypeKept(t *testing.T) {
	t.Parallel()
	in := `!{"type":"stamp","raw":":x:","id":"s1"}`
	got := CleanContent(in, "")
	assert.Equal(t, in, got.Body)
}

func TestCleanContentCodeFenceUntouched(t *testing.T) {
	t.Parallel()
	embed := `!{"type":"user","raw":"@alice","id":"u1"}`
	in := "before\n```\n" + embed + "\n```\nafter"
	got := CleanContent(in, "")
	assert.Contains(t, got.Body, embed)
	assert.Contains(t, got.Body, "before")
	assert.Contains(t, got.Body, "after")
}

func TestCleanContentInlineCodeUntouched(t *testing.T) {
	t.Parallel()
	embed := `!{"type":"user","raw":"@alice","id":"u1"}`
	in := "a `" + embed + "` b"
	got := CleanContent(in, "")
	assert.Equal(t, in, got.Body)
}

func TestCleanContentTailURLStripped(t *testing.T) {
	t.Parallel()
	in := "hello\n" + testOrigin + "/messages/" + testQuoteID1 + "\n" +
		testOrigin + "/files/" + testFileID1
	got := CleanContent(in, testOrigin)
	assert.Equal(t, "hello", got.Body)
	assert.Equal(t, []string{testQuoteID1}, got.QuoteIDs)
	assert.Equal(t, []string{testFileID1}, got.FileIDs)
}

func TestCleanContentInlineURLKept(t *testing.T) {
	t.Parallel()
	in := "see " + testOrigin + "/messages/" + testQuoteID1 + " now"
	got := CleanContent(in, testOrigin)
	assert.Equal(t, in, got.Body)
	assert.Equal(t, []string{testQuoteID1}, got.QuoteIDs)
}

func TestCleanContentNoOriginNoURL(t *testing.T) {
	t.Parallel()
	in := "see " + testOrigin + "/messages/" + testQuoteID1 + " now"
	got := CleanContent(in, "")
	assert.Equal(t, in, got.Body)
	assert.Empty(t, got.QuoteIDs)
}

func TestCleanContentURLInCodeIgnored(t *testing.T) {
	t.Parallel()
	in := "```\n" + testOrigin + "/messages/" + testQuoteID1 + "\n```"
	got := CleanContent(in, testOrigin)
	assert.Equal(t, in, got.Body)
	assert.Empty(t, got.QuoteIDs)
}

func TestRenderQuoteBodyNestedPlaceholder(t *testing.T) {
	t.Parallel()
	in := "hi @" + `!{"type":"message","raw":"q","id":"` + testQuoteID2 + `"}` +
		` !{"type":"file","raw":"f","id":"` + testFileID1 + `"}`
	got := RenderQuoteBody(in)
	assert.Equal(t, "hi @[引用メッセージ] [添付ファイル]", got)
}

func TestRenderQuoteBodyFullDisplay(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for i := 0; i < 30; i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("line")
	}
	got := RenderQuoteBody(b.String())
	assert.Equal(t, b.String(), got)
}

func TestCleanContentWithHighlightDirectMention(t *testing.T) {
	t.Parallel()
	in := `hi !{"type":"user","raw":"@alice","id":"u1"} and !{"type":"user","raw":"@bob","id":"u2"}`
	got := CleanContentWithHighlight(in, "", "u1", nil)
	assert.Equal(t, "hi @alice [to you] and @bob", got.Body)
	assert.True(t, got.SelfMentioned)
}

func TestCleanContentWithHighlightGroupMention(t *testing.T) {
	t.Parallel()
	groups := map[string]struct{}{"g1": {}}
	in := `hi !{"type":"group","raw":"@team","id":"g1"} and !{"type":"group","raw":"@other","id":"g2"}`
	got := CleanContentWithHighlight(in, "", "u1", groups)
	assert.Equal(t, "hi @team [to you] and @other", got.Body)
	assert.True(t, got.SelfMentioned)
}

func TestCleanContentWithHighlightNoSelfMention(t *testing.T) {
	t.Parallel()
	in := `hi !{"type":"user","raw":"@bob","id":"u2"} and !{"type":"channel","raw":"#general","id":"c1"}`
	got := CleanContentWithHighlight(in, "", "u1", map[string]struct{}{"g1": {}})
	assert.Equal(t, "hi @bob and #general", got.Body)
	assert.False(t, got.SelfMentioned)
}

func TestCleanContentWithHighlightCodeUntouched(t *testing.T) {
	t.Parallel()
	embed := `!{"type":"user","raw":"@alice","id":"u1"}`
	in := "before\n```\n" + embed + "\n```\nafter `" + embed + "`"
	got := CleanContentWithHighlight(in, "", "u1", nil)
	assert.Contains(t, got.Body, embed)
	assert.NotContains(t, got.Body, selfMentionMarker)
	assert.False(t, got.SelfMentioned)
}

func TestCleanContentWithHighlightEmptySelfIDNoMarker(t *testing.T) {
	t.Parallel()
	in := `hi !{"type":"user","raw":"@alice","id":"u1"}`
	got := CleanContentWithHighlight(in, "", "", nil)
	assert.Equal(t, "hi @alice", got.Body)
	assert.False(t, got.SelfMentioned)
}

func TestRenderQuoteBodyWithHighlight(t *testing.T) {
	t.Parallel()
	groups := map[string]struct{}{"g1": {}}
	in := `hi !{"type":"user","raw":"@alice","id":"u1"} and !{"type":"group","raw":"@team","id":"g1"} and !{"type":"user","raw":"@bob","id":"u2"}`
	got := RenderQuoteBodyWithHighlight(in, "u1", groups)
	assert.Equal(t, "hi @alice [to you] and @team [to you] and @bob", got)
}

func TestRenderQuoteBodyWithHighlightNestedPlaceholder(t *testing.T) {
	t.Parallel()
	in := `hi !{"type":"user","raw":"@alice","id":"u1"}` +
		` !{"type":"message","raw":"q","id":"` + testQuoteID2 + `"}`
	got := RenderQuoteBodyWithHighlight(in, "u1", nil)
	assert.Equal(t, "hi @alice [to you] [引用メッセージ]", got)
}
