package view

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/moznion/go-optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/traPtitech/go-traq"
)

const (
	testQuoteID1 = "11111111-1111-1111-1111-111111111111"
	testFileID1  = "33333333-3333-3333-3333-333333333333"
	testOrigin   = "https://q.trap.jp"
)

func TestWriteMessageListOrderAndFormat(t *testing.T) {
	t.Parallel()
	loc := time.UTC
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	users := map[string]traq.User{
		"u1": {Id: "u1", Name: "alice", DisplayName: "Alice Smith"},
		"u2": {Id: "u2", Name: "bob", DisplayName: "bob"},
	}
	// 取得順は desc (新しい順) を想定
	msgs := []traq.Message{
		{
			Id:        "m2",
			UserId:    "u2",
			Content:   "newer\nlong line",
			CreatedAt: base.Add(time.Minute),
			UpdatedAt: base.Add(2 * time.Minute),
			Pinned:    true,
		},
		{Id: "m1", UserId: "u1", Content: "older", CreatedAt: base, UpdatedAt: base},
	}

	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Offset: 0, Order: "desc",
		Msgs: msgs, Users: users, Loc: loc,
		Stamps: map[string]string{},
	}))

	want := strings.Join([]string{
		"#general (2件 limit=10 offset=0 order=desc)",
		"",
		"2026-10-17T10:00:00Z @alice (Alice Smith) id:m1",
		"  older",
		"",
		"2026-10-17T10:01:00Z @bob id:m2 [edited] [pinned]",
		"  newer",
		"  long line",
		"",
	}, "\n")
	assert.Equal(t, want, buf.String())
}

func TestWriteMessageListStamps(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	msgs := []traq.Message{{
		Id: "m1", UserId: "u1", Content: "hello",
		CreatedAt: base, UpdatedAt: base,
		Stamps: []traq.MessageStamp{
			{StampId: "s2", Count: 3},
			{StampId: "s1", Count: 1},
		},
	}}

	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Order: "desc",
		Msgs: msgs,
		Users: map[string]traq.User{
			"u1": {Id: "u1", Name: "alice", DisplayName: "alice"},
		},
		Stamps: map[string]string{"s1": "ok", "s2": "thank"},
		Loc:    time.UTC,
	}))

	want := strings.Join([]string{
		"#general (1件 limit=10 offset=0 order=desc)",
		"",
		"2026-10-17T10:00:00Z @alice id:m1",
		"  hello",
		"スタンプ",
		"  :thank: ×3",
		"  :ok: ×1",
		"",
	}, "\n")
	assert.Equal(t, want, buf.String())
}

func TestFormatStampItems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stamps []traq.MessageStamp
		names  map[string]string
		want   []string
	}{
		{
			name:   "empty",
			stamps: nil,
			names:  map[string]string{},
			want:   nil,
		},
		{
			name: "preserves API order",
			stamps: []traq.MessageStamp{
				{StampId: "s2", Count: 3},
				{StampId: "s1", Count: 1},
			},
			names: map[string]string{"s1": "ok", "s2": "thank"},
			want:  []string{":thank: ×3", ":ok: ×1"},
		},
		{
			name: "unknown stamp falls back to ID",
			stamps: []traq.MessageStamp{
				{StampId: "s9", Count: 2},
			},
			names: map[string]string{},
			want:  []string{":s9: ×2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, FormatStampItems(tt.stamps, tt.names))
		})
	}
}

func TestWriteMessageListUnreadDivider(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	users := map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}}
	msgs := []traq.Message{
		{Id: "m1", UserId: "u1", Content: "read", CreatedAt: base, UpdatedAt: base},
		{
			Id:        "m2",
			UserId:    "u1",
			Content:   "unread",
			CreatedAt: base.Add(time.Minute),
			UpdatedAt: base.Add(time.Minute),
		},
	}

	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label:  "#general",
		Limit:  10,
		Order:  "desc",
		Msgs:   msgs,
		Users:  users,
		Stamps: map[string]string{},
		Loc:    time.UTC,
		Unread: optional.Some(traq.UnreadChannel{
			ChannelId:       "c1",
			Count:           5,
			Noticeable:      true,
			OldestMessageId: "m2",
		}),
	}))

	out := buf.String()
	dividerIdx := strings.Index(out, "----- ここから未読 (5件) [mentionあり] -----")
	m2Idx := strings.Index(out, "id:m2")
	require.NotEqual(t, -1, dividerIdx, "divider not found: %s", out)
	require.NotEqual(t, -1, m2Idx, "m2 not found: %s", out)
	assert.Less(t, dividerIdx, m2Idx, "divider should come before the first unread message")
	assert.Contains(t, out, "id:m1")
}

func TestWriteMessageListUnreadNotInPage(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general",
		Limit: 10,
		Order: "desc",
		Msgs: []traq.Message{
			{Id: "m1", UserId: "u1", Content: "hi", CreatedAt: base, UpdatedAt: base},
		},
		Users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		Stamps: map[string]string{},
		Loc:    time.UTC,
		Unread: optional.Some(
			traq.UnreadChannel{ChannelId: "c1", Count: 3, OldestMessageId: "other"},
		),
	}))
	assert.Contains(t, buf.String(), "# 未読 3 件 (このページに未読の先頭は含まれない)")
}

func TestWriteMessageListUnreadOnly(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	users := map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}}
	msgs := []traq.Message{
		{Id: "m1", UserId: "u1", Content: "unread-1", CreatedAt: base, UpdatedAt: base},
		{
			Id:        "m2",
			UserId:    "u1",
			Content:   "unread-2",
			CreatedAt: base.Add(time.Minute),
			UpdatedAt: base.Add(time.Minute),
		},
	}

	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label:  "#general",
		Limit:  10,
		Order:  "desc",
		Msgs:   msgs,
		Users:  users,
		Stamps: map[string]string{},
		Loc:    time.UTC,
		Unread: optional.Some(traq.UnreadChannel{
			ChannelId:       "c1",
			Count:           7,
			Noticeable:      true,
			OldestMessageId: "m1",
		}),
		UnreadOnly: true,
	}))

	out := buf.String()
	assert.Contains(t, out, "#general (未読7件中 2件 limit=10 offset=0 order=desc)")
	assert.NotContains(t, out, "ここから未読", "the divider should be suppressed in unread-only mode")
	assert.NotContains(t, out, "このページに未読の先頭は含まれない")
	i1 := strings.Index(out, "id:m1")
	i2 := strings.Index(out, "id:m2")
	require.NotEqual(t, -1, i1)
	require.NotEqual(t, -1, i2)
	assert.Less(t, i1, i2, "messages should be displayed oldest first")
}

func TestWriteMessageListUnreadOnlyPartial(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general",
		Limit: 2,
		Order: "desc",
		Msgs: []traq.Message{
			{Id: "m1", UserId: "u1", Content: "hi", CreatedAt: base, UpdatedAt: base},
		},
		Users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		Stamps: map[string]string{},
		Loc:    time.UTC,
		Unread: optional.Some(traq.UnreadChannel{
			ChannelId:       "c1",
			Count:           5,
			OldestMessageId: "m9",
		}),
		UnreadOnly: true,
	}))
	assert.Contains(t, buf.String(), "#general (未読5件中 1件 limit=2 offset=0 order=desc)")
}

func TestWriteMessageListUnreadOnlyNoUnread(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label:      "#general",
		Limit:      10,
		Order:      "desc",
		Msgs:       nil,
		Users:      map[string]traq.User{},
		Stamps:     map[string]string{},
		Loc:        time.UTC,
		UnreadOnly: true,
	}))
	assert.Equal(t, "#general (未読0件)\n\n", buf.String())
}

func TestWriteMessageListUserNotFoundFallback(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	err := WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Order: "desc",
		Msgs:   []traq.Message{{Id: "m1", UserId: "missing", CreatedAt: base, UpdatedAt: base}},
		Users:  map[string]traq.User{},
		Stamps: map[string]string{}, Loc: time.UTC,
	})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "(ユーザー解決失敗 id:missing)")
	assert.NotContains(
		t,
		buf.String(),
		"@unknown",
		"fallback must not collide with a real user name",
	)
}

func TestFormatQuoteUserNotFoundFallback(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	ml := MessageList{
		Users: map[string]traq.User{},
		Loc:   time.UTC,
		Quoted: map[string]traq.Message{
			"q1": {
				Id:        "q1",
				UserId:    "gone",
				Content:   "quoted text",
				CreatedAt: base,
				UpdatedAt: base,
			},
		},
	}
	lines := FormatQuote("q1", ml)
	out := strings.Join(lines, "\n")
	assert.Contains(t, out, "(ユーザー解決失敗 id:gone)")
	assert.NotContains(
		t,
		out,
		"@unknown",
		"fallback must not collide with a real user name",
	)
	assert.Contains(t, out, "quoted text")
}

func TestWriteMessageListLocalTimezone(t *testing.T) {
	t.Parallel()
	jst := time.FixedZone("JST", 9*60*60)
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Order: "desc",
		Msgs: []traq.Message{
			{Id: "m1", UserId: "u1", Content: "hi", CreatedAt: base, UpdatedAt: base},
		},
		Users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		Stamps: map[string]string{}, Loc: jst,
	}))
	assert.Contains(t, buf.String(), "2026-10-17T19:00:00+09:00")
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
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Offset: 0, Order: "desc",
		Msgs: []traq.Message{{
			Id: "m1", UserId: "u1", Content: content,
			CreatedAt: base, UpdatedAt: base,
			Stamps: []traq.MessageStamp{{StampId: "s1", Count: 2}},
		}},
		Users:  users,
		Stamps: map[string]string{"s1": "ok"},
		Loc:    time.UTC,
		Origin: testOrigin,
		Quoted: map[string]traq.Message{testQuoteID1: {
			Id: "m2", UserId: "u2", Content: "quoted body",
			CreatedAt: qtime, UpdatedAt: qtime,
		}},
		Files: map[string]traq.FileInfo{testFileID1: {
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
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Offset: 0, Order: "desc",
		Msgs: []traq.Message{{
			Id: "m1", UserId: "u1", Content: content,
			CreatedAt: base, UpdatedAt: base,
		}},
		Users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		Stamps: map[string]string{},
		Loc:    time.UTC,
		Quoted: map[string]traq.Message{},
		Files:  map[string]traq.FileInfo{},
	}))
	assert.Contains(t, buf.String(), "引用 (取得失敗 id:"+testQuoteID1+")")
}

func TestFormatMessageFileSection(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	content := "doc\n" +
		`!{"type":"file","raw":"r","id":"` + testFileID1 + `"}`
	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Offset: 0, Order: "desc",
		Msgs: []traq.Message{{
			Id: "m1", UserId: "u1", Content: content,
			CreatedAt: base, UpdatedAt: base,
		}},
		Users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		Stamps: map[string]string{},
		Loc:    time.UTC,
		Quoted: map[string]traq.Message{},
		Files: map[string]traq.FileInfo{testFileID1: {
			Id: testFileID1, Name: "report.pdf", Mime: "application/pdf", Size: 12345,
		}},
	}))
	out := buf.String()
	assert.Contains(t, out, "添付ファイル")
	assert.Contains(t, out, "  report.pdf (application/pdf, 12345 bytes, id:"+testFileID1+")")
}

func TestWriteMessageListSearchHeader(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	total := int64(42)
	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "search \"foo\"", Limit: 10, Offset: 0, Sort: "-createdAt",
		Msgs: []traq.Message{
			{Id: "m1", UserId: "u1", Content: "hello foo", CreatedAt: base, UpdatedAt: base},
		},
		Users:     map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		Stamps:    map[string]string{},
		Loc:       time.UTC,
		TotalHits: optional.Some(total),
	}))
	out := buf.String()
	assert.Contains(t, out, "search \"foo\" (全42件中 1件 limit=10 offset=0 sort=-createdAt)")
	assert.Contains(t, out, "hello foo")
}

func TestWriteMessageListSelfMention(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	content := `hi !{"type":"user","raw":"@alice","id":"me"}` +
		` and !{"type":"user","raw":"@bob","id":"u2"}` +
		` and !{"type":"group","raw":"@team","id":"g1"}` +
		` and !{"type":"group","raw":"@other","id":"g2"}`
	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Order: "desc",
		Msgs: []traq.Message{{
			Id: "m1", UserId: "u2", Content: content,
			CreatedAt: base, UpdatedAt: base,
		}},
		Users: map[string]traq.User{
			"me": {Id: "me", Name: "alice", DisplayName: "alice"},
			"u2": {Id: "u2", Name: "bob", DisplayName: "bob"},
		},
		Stamps:   map[string]string{},
		Loc:      time.UTC,
		SelfID:   "me",
		MyGroups: map[string]struct{}{"g1": {}},
	}))
	out := buf.String()
	assert.Contains(t, out, "id:m1 [to you]")
	assert.Contains(t, out, "hi @alice [to you] and @bob and @team [to you] and @other")
}

func TestWriteMessageListNoSelfMention(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	content := `hi !{"type":"user","raw":"@bob","id":"u2"}` +
		` and !{"type":"group","raw":"@other","id":"g2"}`
	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Order: "desc",
		Msgs: []traq.Message{{
			Id: "m1", UserId: "u2", Content: content,
			CreatedAt: base, UpdatedAt: base,
		}},
		Users:    map[string]traq.User{"u2": {Id: "u2", Name: "bob", DisplayName: "bob"}},
		Stamps:   map[string]string{},
		Loc:      time.UTC,
		SelfID:   "me",
		MyGroups: map[string]struct{}{"g1": {}},
	}))
	out := buf.String()
	assert.NotContains(t, out, "[to you]")
	assert.Contains(t, out, "hi @bob and @other")
}

func TestFormatQuoteSelfMentionDoesNotLightHeader(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	content := `!{"type":"message","raw":"q","id":"` + testQuoteID1 + `"}`
	quotedContent := `hi !{"type":"user","raw":"@alice","id":"me"}`
	var buf bytes.Buffer
	require.NoError(t, WriteMessageList(&buf, MessageList{
		Label: "#general", Limit: 10, Offset: 0, Order: "desc",
		Msgs: []traq.Message{{
			Id: "m1", UserId: "u2", Content: content,
			CreatedAt: base, UpdatedAt: base,
		}},
		Users: map[string]traq.User{
			"me": {Id: "me", Name: "alice", DisplayName: "alice"},
			"u2": {Id: "u2", Name: "bob", DisplayName: "bob"},
		},
		Stamps: map[string]string{},
		Loc:    time.UTC,
		Quoted: map[string]traq.Message{testQuoteID1: {
			Id: "q1", UserId: "u2", Content: quotedContent,
			CreatedAt: base, UpdatedAt: base,
		}},
		SelfID: "me",
	}))
	out := buf.String()
	assert.Contains(t, out, "引用 @bob 2026-10-17T10:00:00Z id:q1")
	assert.Contains(t, out, "  hi @alice [to you]")
	assert.NotContains(t, out, "id:m1 [to you]", "header must not light up from a quoted mention")
}
