package message

import (
	"errors"
	"testing"

	"github.com/moznion/go-optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
)

func validSearchOptions() SearchOptions {
	return SearchOptions{Limit: 10, Offset: 0, Sort: DefaultSearchSort}
}

func TestService_Search(t *testing.T) {
	t.Parallel()

	hits := &gotraq.MessageSearchResult{
		TotalHits: 1,
		Hits:      []gotraq.Message{{Id: "m1", Content: "hello foo"}},
	}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().SearchMessages(gomock.Any(), gomock.Any()).Return(hits, nil)

		res, err := NewService(mc).Search(t.Context(), "foo", validSearchOptions())
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.TotalHits)
		require.Len(t, res.Msgs, 1)
	})

	t.Run("invalid flags", func(t *testing.T) {
		t.Parallel()
		cases := map[string]SearchOptions{
			"limit-low":  {Limit: 0, Sort: DefaultSearchSort},
			"limit-high": {Limit: 101, Sort: DefaultSearchSort},
			"sort":       {Limit: 10, Sort: "asc"},
			"citation":   {Limit: 10, Sort: DefaultSearchSort, Citation: "not-a-uuid"},
		}
		for name, opts := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				mc := traq.NewMockClient(gomock.NewController(t))
				_, err := NewService(mc).Search(t.Context(), "foo", opts)
				var ec *exit.Error
				require.ErrorAs(t, err, &ec)
				assert.Equal(t, exit.Usage, ec.Code)
			})
		}
	})

	t.Run("in channel not found", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#does-not-exist").
			Return("", exit.Generalf("channel not found"))

		opts := validSearchOptions()
		opts.In = "#does-not-exist"
		_, err := NewService(mc).Search(t.Context(), "foo", opts)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
	})

	t.Run("from user not found is usage", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveUserIDs(gomock.Any(), []string{"@no-such-user-xyz"}).
			Return(nil, errors.New("user not found: @no-such-user-xyz"))

		opts := validSearchOptions()
		opts.From = []string{"@no-such-user-xyz"}
		_, err := NewService(mc).Search(t.Context(), "foo", opts)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
	})

	t.Run("empty hits skip enrichment", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().SearchMessages(gomock.Any(), gomock.Any()).Return(
			&gotraq.MessageSearchResult{TotalHits: 0, Hits: []gotraq.Message{}}, nil,
		)

		res, err := NewService(mc).Search(t.Context(), "foo", validSearchOptions())
		require.NoError(t, err)
		assert.Empty(t, res.Msgs)
	})

	t.Run("word option mapping", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().SearchMessages(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ any, q traq.SearchQuery) (*gotraq.MessageSearchResult, error) {
				word, err := q.Word.Take()
				require.NoError(t, err)
				assert.Equal(t, "foo", word)
				_ = optional.None[string]()
				return hits, nil
			},
		)

		_, err := NewService(mc).Search(t.Context(), "foo", validSearchOptions())
		require.NoError(t, err)
	})
}
