package traq

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/go-traq"
)

func TestBuildPathMap(t *testing.T) {
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

func TestFormatChannelLabel(t *testing.T) {
	t.Parallel()
	parentID := "p1"
	list := &traq.ChannelList{
		Public: []traq.Channel{
			{Id: "p1", Name: "general"},
			{Id: "c1", Name: "sub", ParentId: *traq.NewNullableString(&parentID)},
		},
		Dm: []traq.DMChannel{
			{Id: "dm1", UserId: "u1"},
			{Id: "dm2", UserId: "u-missing"},
		},
	}
	userNames := map[string]string{"u1": "alice"}

	assert.Equal(t, "#general", FormatChannelLabel(list, userNames, "p1"))
	assert.Equal(t, "#general/sub", FormatChannelLabel(list, userNames, "c1"))
	assert.Equal(t, "@alice", FormatChannelLabel(list, userNames, "dm1"))
	assert.Equal(
		t,
		"(ユーザー解決失敗 id:u-missing)",
		FormatChannelLabel(list, userNames, "dm2"),
	)
	assert.Equal(t, "unknown-id", FormatChannelLabel(list, userNames, "unknown-id"))
	assert.Equal(t, "unknown-id", FormatChannelLabel(nil, userNames, "unknown-id"))
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
