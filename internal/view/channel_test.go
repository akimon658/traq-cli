package view

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akimon658/traq-cli/internal/channel"
)

func TestWriteUnreadChannels(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	g := channel.Groups{
		DM:      []channel.Entry{{ChannelID: "dm1", Display: "@alice", Count: 2, UpdatedAt: base}},
		Mention: []channel.Entry{{ChannelID: "c1", Display: "#general", Count: 3, UpdatedAt: base}},
	}
	var buf bytes.Buffer
	require.NoError(t, WriteUnreadChannels(&buf, g))
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
	require.NoError(t, WriteUnreadChannels(&buf, channel.Groups{}))
	assert.Contains(t, buf.String(), "未読チャンネルはありません")
}
