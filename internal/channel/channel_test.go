package channel

import (
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
	m := BuildPathMap(list)
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

	g, err := Classify(unreads, pathMap, dmMap, userNames, subLevels, stars)
	require.NoError(t, err)

	require.Len(t, g.DM, 1)
	assert.Equal(t, "@alice", g.DM[0].Display)

	require.Len(t, g.Mention, 2)
	// グループ内は updatedAt 降順
	assert.Equal(t, "#mention-notify", g.Mention[0].Display)
	assert.Equal(t, "#mention", g.Mention[1].Display)

	require.Len(t, g.Notified, 2)
	assert.Equal(t, "#both", g.Notified[0].Display)
	assert.Equal(t, "#notify", g.Notified[1].Display)

	require.Len(t, g.Starred, 1)
	assert.Equal(t, "#star", g.Starred[0].Display)

	require.Len(t, g.Others, 1)
	assert.Equal(t, "#other", g.Others[0].Display)
}

func TestClassifyUnreadsErrors(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	_, err := Classify(
		[]traq.UnreadChannel{{ChannelId: "unknown", Count: 1, UpdatedAt: base}},
		map[string]string{},
		map[string]string{},
		map[string]string{},
		map[string]traq.ChannelSubscribeLevel{},
		map[string]bool{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel not found")
}

func TestClassifyUnreadsDMUserNotFoundFallback(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	g, err := Classify(
		[]traq.UnreadChannel{{ChannelId: "dm1", Count: 1, UpdatedAt: base}},
		map[string]string{},
		map[string]string{"dm1": "u-missing"},
		map[string]string{},
		map[string]traq.ChannelSubscribeLevel{},
		map[string]bool{},
	)
	require.NoError(t, err)
	require.Len(t, g.DM, 1)
	assert.Equal(t, "(ユーザー解決失敗 id:u-missing)", g.DM[0].Display)
	assert.NotContains(t, g.DM[0].Display, "@", "fallback must not look like a user handle")
}

func TestFindByPath(t *testing.T) {
	t.Parallel()
	parentID := "p1"
	list := &traq.ChannelList{
		Public: []traq.Channel{
			{Id: "p1", Name: "general"},
			{Id: "c1", Name: "sub", ParentId: *traq.NewNullableString(&parentID)},
		},
	}
	id, err := FindByPath(list, "general")
	require.NoError(t, err)
	assert.Equal(t, "p1", id)
	id, err = FindByPath(list, "#general/sub")
	require.NoError(t, err)
	assert.Equal(t, "c1", id)
	_, err = FindByPath(list, "does-not-exist")
	require.Error(t, err)
}
