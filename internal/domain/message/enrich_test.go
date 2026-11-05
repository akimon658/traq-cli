package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/traq"
)

func TestService_GetEnrichment(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetSelfIDAndGroups(gomock.Any()).
			Return("me", map[string]struct{}{"g1": {}}, nil)
		mc.EXPECT().GetUsers(gomock.Any()).Return(
			[]gotraq.User{{Id: "u1", Name: "alice"}}, nil,
		)
		mc.EXPECT().GetStampNameMap(gomock.Any()).Return(map[string]string{"s1": "ok"}, nil)
		mc.EXPECT().Origin().Return("https://q.trap.jp").AnyTimes()

		enrich, err := NewService(mc).GetEnrichment(t.Context(), []string{"hello"})
		require.NoError(t, err)
		assert.Equal(t, "alice", enrich.Users["u1"].Name)
		assert.Equal(t, "me", enrich.SelfID)
	})

	t.Run("users failure is error", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetSelfIDAndGroups(gomock.Any()).
			Return("", map[string]struct{}{}, nil)
		mc.EXPECT().GetUsers(gomock.Any()).Return(nil, errors.New("users failed"))

		_, err := NewService(mc).GetEnrichment(t.Context(), []string{"hello"})
		require.Error(t, err)
	})

	t.Run("stamps failure is error", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetSelfIDAndGroups(gomock.Any()).
			Return("", map[string]struct{}{}, nil)
		mc.EXPECT().GetUsers(gomock.Any()).Return([]gotraq.User{}, nil)
		mc.EXPECT().GetStampNameMap(gomock.Any()).Return(nil, errors.New("stamps failed"))

		_, err := NewService(mc).GetEnrichment(t.Context(), []string{"hello"})
		require.Error(t, err)
	})

	t.Run("self failure continues empty", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetSelfIDAndGroups(gomock.Any()).
			Return("", map[string]struct{}{}, errors.New("me failed"))
		mc.EXPECT().GetUsers(gomock.Any()).Return([]gotraq.User{}, nil)
		mc.EXPECT().GetStampNameMap(gomock.Any()).Return(map[string]string{}, nil)
		mc.EXPECT().Origin().Return("https://q.trap.jp").AnyTimes()

		enrich, err := NewService(mc).GetEnrichment(t.Context(), []string{"hello"})
		require.NoError(t, err)
		assert.Empty(t, enrich.SelfID)
	})

	t.Run("quote and file failures are ignored", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		stubEnrichment(mc)
		quoteID := "11111111-1111-1111-1111-111111111111"
		fileID := "33333333-3333-3333-3333-333333333333"
		mc.EXPECT().GetMessage(gomock.Any(), quoteID).Return(nil, errors.New("gone"))
		mc.EXPECT().GetFileMeta(gomock.Any(), fileID).Return(nil, errors.New("gone"))

		content := "q " + `!{"type":"message","raw":"q","id":"` + quoteID + `"}` +
			" f " + `!{"type":"file","raw":"r","id":"` + fileID + `"}`
		enrich, err := NewService(mc).GetEnrichment(t.Context(), []string{content})
		require.NoError(t, err)
		assert.Empty(t, enrich.Quoted)
		assert.Empty(t, enrich.Files)
	})
}
