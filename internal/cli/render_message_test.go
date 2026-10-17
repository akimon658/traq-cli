package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/go-traq"
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
	got := cleanContent(in, "")
	assert.Equal(t, "hello @alice and #general and @team", got.body)
	assert.Empty(t, got.quoteIDs)
	assert.Empty(t, got.fileIDs)
}

func TestCleanContentMessageFileJSON(t *testing.T) {
	t.Parallel()
	in := "see this\n" +
		`!{"type":"message","raw":"引用","id":"` + testQuoteID1 + `"}` + "\n" +
		`file !{"type":"file","raw":"a.png","id":"` + testFileID1 + `"} end`
	got := cleanContent(in, "")
	assert.Equal(t, "see this\n\nfile  end", got.body)
	require.Equal(t, []string{testQuoteID1}, got.quoteIDs)
	require.Equal(t, []string{testFileID1}, got.fileIDs)
}

func TestCleanContentJSONDedupOrder(t *testing.T) {
	t.Parallel()
	in := `!{"type":"message","raw":"x","id":"` + testQuoteID1 + `"} ` +
		`!{"type":"message","raw":"x","id":"` + testQuoteID2 + `"} ` +
		`!{"type":"message","raw":"x","id":"` + testQuoteID1 + `"}`
	got := cleanContent(in, "")
	assert.Equal(t, []string{testQuoteID1, testQuoteID2}, got.quoteIDs)
}

func TestCleanContentBrokenJSONKept(t *testing.T) {
	t.Parallel()
	in := `!{"type":"user","raw":"@alice"`
	got := cleanContent(in, "")
	assert.Equal(t, in, got.body)
	assert.Empty(t, got.quoteIDs)
}

func TestCleanContentUnknownTypeKept(t *testing.T) {
	t.Parallel()
	in := `!{"type":"stamp","raw":":x:","id":"s1"}`
	got := cleanContent(in, "")
	assert.Equal(t, in, got.body)
}

func TestCleanContentCodeFenceUntouched(t *testing.T) {
	t.Parallel()
	embed := `!{"type":"user","raw":"@alice","id":"u1"}`
	in := "before\n```\n" + embed + "\n```\nafter"
	got := cleanContent(in, "")
	assert.Contains(t, got.body, embed)
	assert.Contains(t, got.body, "before")
	assert.Contains(t, got.body, "after")
}

func TestCleanContentInlineCodeUntouched(t *testing.T) {
	t.Parallel()
	embed := `!{"type":"user","raw":"@alice","id":"u1"}`
	in := "a `" + embed + "` b"
	got := cleanContent(in, "")
	assert.Equal(t, in, got.body)
}

func TestCleanContentTailURLStripped(t *testing.T) {
	t.Parallel()
	in := "hello\n" + testOrigin + "/messages/" + testQuoteID1 + "\n" +
		testOrigin + "/files/" + testFileID1
	got := cleanContent(in, testOrigin)
	assert.Equal(t, "hello", got.body)
	assert.Equal(t, []string{testQuoteID1}, got.quoteIDs)
	assert.Equal(t, []string{testFileID1}, got.fileIDs)
}

func TestCleanContentInlineURLKept(t *testing.T) {
	t.Parallel()
	in := "see " + testOrigin + "/messages/" + testQuoteID1 + " now"
	got := cleanContent(in, testOrigin)
	assert.Equal(t, in, got.body)
	assert.Equal(t, []string{testQuoteID1}, got.quoteIDs)
}

func TestCleanContentNoOriginNoURL(t *testing.T) {
	t.Parallel()
	in := "see " + testOrigin + "/messages/" + testQuoteID1 + " now"
	got := cleanContent(in, "")
	assert.Equal(t, in, got.body)
	assert.Empty(t, got.quoteIDs)
}

func TestCleanContentURLInCodeIgnored(t *testing.T) {
	t.Parallel()
	in := "```\n" + testOrigin + "/messages/" + testQuoteID1 + "\n```"
	got := cleanContent(in, testOrigin)
	assert.Equal(t, in, got.body)
	assert.Empty(t, got.quoteIDs)
}

func TestRenderQuoteBodyNestedPlaceholder(t *testing.T) {
	t.Parallel()
	in := "hi @" + `!{"type":"message","raw":"q","id":"` + testQuoteID2 + `"}` +
		` !{"type":"file","raw":"f","id":"` + testFileID1 + `"}`
	got := renderQuoteBody(in)
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
	got := renderQuoteBody(b.String())
	assert.Equal(t, b.String(), got)
}

func TestFormatMessageFullBlock(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	qtime := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	users := map[string]traq.User{
		"u1": {Id: "u1", Name: "alice", DisplayName: "alice"},
		"u2": {Id: "u2", Name: "carol", DisplayName: "Carol"},
	}
	content := "hi " + `!{"type":"user","raw":"@carol","id":"u2"}` + "\n" +
		`!{"type":"message","raw":"q","id":"` + testQuoteID1 + `"}`
	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label: "#general", limit: 10, offset: 0, order: "desc",
		msgs: []traq.Message{{
			Id: "m1", UserId: "u1", Content: content,
			CreatedAt: base, UpdatedAt: base,
			Stamps: []traq.MessageStamp{{StampId: "s1", Count: 2}},
		}},
		users:  users,
		stamps: map[string]string{"s1": "ok"},
		loc:    time.UTC,
		origin: testOrigin,
		quoted: map[string]traq.Message{testQuoteID1: {
			Id: "m2", UserId: "u2", Content: "quoted body",
			CreatedAt: qtime, UpdatedAt: qtime,
		}},
		files: map[string]traq.FileInfo{testFileID1: {
			Id: testFileID1, Name: "report.pdf", Mime: "application/pdf", Size: 12345,
		}},
	}))
	out := buf.String()
	assert.Contains(t, out, "2026-10-17T10:00:00Z @alice id:m1")
	assert.Contains(t, out, "  hi @carol")
	assert.Contains(t, out, "引用 @carol (Carol) 2026-10-16T09:00:00Z id:m2")
	assert.Contains(t, out, "  quoted body")
	assert.NotContains(t, out, `"type"`)
}

func TestFormatMessageQuoteFetchFailure(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	content := `!{"type":"message","raw":"q","id":"` + testQuoteID1 + `"}`
	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label: "#general", limit: 10, offset: 0, order: "desc",
		msgs: []traq.Message{{
			Id: "m1", UserId: "u1", Content: content,
			CreatedAt: base, UpdatedAt: base,
		}},
		users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		stamps: map[string]string{},
		loc:    time.UTC,
		quoted: map[string]traq.Message{},
		files:  map[string]traq.FileInfo{},
	}))
	assert.Contains(t, buf.String(), "引用 (取得失敗 id:"+testQuoteID1+")")
}

func TestFormatMessageFileSection(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	content := "doc\n" +
		`!{"type":"file","raw":"r","id":"` + testFileID1 + `"}`
	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label: "#general", limit: 10, offset: 0, order: "desc",
		msgs: []traq.Message{{
			Id: "m1", UserId: "u1", Content: content,
			CreatedAt: base, UpdatedAt: base,
		}},
		users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		stamps: map[string]string{},
		loc:    time.UTC,
		quoted: map[string]traq.Message{},
		files: map[string]traq.FileInfo{testFileID1: {
			Id: testFileID1, Name: "report.pdf", Mime: "application/pdf", Size: 12345,
		}},
	}))
	out := buf.String()
	assert.Contains(t, out, "添付ファイル")
	assert.Contains(t, out, "  report.pdf (application/pdf, 12345 bytes, id:"+testFileID1+")")
}
