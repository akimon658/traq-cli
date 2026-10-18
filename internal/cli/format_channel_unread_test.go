package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/go-traq"
)

func TestBuildChannelPathMap(t *testing.T) {
	t.Parallel()
	parentID := "p1"
	list := &traq.ChannelList{
		Public: []traq.Channel{
			{Id: "p1", Name: "general"},
			{Id: "c1", Name: "sub", ParentId: *traq.NewNullableString(&parentID)},
		},
	}
	m := buildChannelPathMap(list)
	assert.Equal(t, "#general", m["p1"])
	assert.Equal(t, "#general/sub", m["c1"])
}

func TestClassifyUnreads(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	unreads := []traq.UnreadChannel{
		{ChannelId: "c-other", Count: 1, Noticeable: false, UpdatedAt: base},
		{ChannelId: "c-star", Count: 2, Noticeable: false, UpdatedAt: base.Add(time.Minute)},
		{ChannelId: "c-notify", Count: 3, Noticeable: false, UpdatedAt: base.Add(2 * time.Minute)},
		{ChannelId: "c-mention", Count: 4, Noticeable: true, UpdatedAt: base.Add(3 * time.Minute)},
		{ChannelId: "dm1", Count: 5, Noticeable: false, UpdatedAt: base.Add(4 * time.Minute)},
		// 通知オンかつお気に入り・メンションかつ通知オンは上位グループに残る
		{ChannelId: "c-both", Count: 6, Noticeable: false, UpdatedAt: base.Add(5 * time.Minute)},
		{
			ChannelId:  "c-mention-notify",
			Count:      7,
			Noticeable: true,
			UpdatedAt:  base.Add(6 * time.Minute),
		},
	}
	pathMap := map[string]string{
		"c-other":          "#other",
		"c-star":           "#star",
		"c-notify":         "#notify",
		"c-mention":        "#mention",
		"c-both":           "#both",
		"c-mention-notify": "#mention-notify",
	}
	dmMap := map[string]string{"dm1": "u1"}
	userNames := map[string]string{"u1": "alice"}
	subLevels := map[string]traq.ChannelSubscribeLevel{
		"c-notify":         traq.CHANNELSUBSCRIBELEVEL_notified,
		"c-both":           traq.CHANNELSUBSCRIBELEVEL_notified,
		"c-mention-notify": traq.CHANNELSUBSCRIBELEVEL_notified,
	}
	stars := map[string]bool{"c-star": true, "c-both": true}

	g, err := classifyUnreads(unreads, pathMap, dmMap, userNames, subLevels, stars)
	require.NoError(t, err)

	require.Len(t, g.dm, 1)
	assert.Equal(t, "@alice", g.dm[0].display)

	require.Len(t, g.mention, 2)
	// グループ内は updatedAt 降順
	assert.Equal(t, "#mention-notify", g.mention[0].display)
	assert.Equal(t, "#mention", g.mention[1].display)

	require.Len(t, g.notified, 2)
	assert.Equal(t, "#both", g.notified[0].display)
	assert.Equal(t, "#notify", g.notified[1].display)

	require.Len(t, g.starred, 1)
	assert.Equal(t, "#star", g.starred[0].display)

	require.Len(t, g.others, 1)
	assert.Equal(t, "#other", g.others[0].display)
}

func TestClassifyUnreadsErrors(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	_, err := classifyUnreads(
		[]traq.UnreadChannel{{ChannelId: "unknown", Count: 1, UpdatedAt: base}},
		map[string]string{},
		map[string]string{},
		map[string]string{},
		map[string]traq.ChannelSubscribeLevel{},
		map[string]bool{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel not found")

	_, err = classifyUnreads(
		[]traq.UnreadChannel{{ChannelId: "dm1", Count: 1, UpdatedAt: base}},
		map[string]string{},
		map[string]string{"dm1": "u-missing"},
		map[string]string{},
		map[string]traq.ChannelSubscribeLevel{},
		map[string]bool{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestWriteUnreadChannels(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	g := unreadGroups{
		dm:      []unreadEntry{{channelID: "dm1", display: "@alice", count: 2, updatedAt: base}},
		mention: []unreadEntry{{channelID: "c1", display: "#general", count: 3, updatedAt: base}},
	}
	var buf bytes.Buffer
	require.NoError(t, writeUnreadChannels(&buf, g))
	out := buf.String()
	assert.Contains(t, out, "DM (1チャンネル)")
	assert.Contains(t, out, "@alice (2件)")
	assert.Contains(t, out, "メンション (1チャンネル)")
	assert.Contains(t, out, "#general (3件)")
	assert.NotContains(t, out, "通知オン")
}

func TestWriteUnreadChannelsEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, writeUnreadChannels(&buf, unreadGroups{}))
	assert.Contains(t, buf.String(), "未読チャンネルはありません")
}

func TestChannelUnreadRejectsArgs(t *testing.T) {
	t.Parallel()
	e := botEnv(t, "")
	_, err := runCLI(t, e, "channel", "unread", "extra")
	require.Error(t, err)
}
