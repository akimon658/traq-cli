package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wasmTestOrigin = "https://q.trap.jp"

func newWASMParserForTest(t *testing.T) *WASMParser {
	t.Helper()
	p, err := NewWASMParser(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = p.Close(t.Context())
	})
	return p
}

func TestNewWASMParser(t *testing.T) {
	t.Run("bundled runtime", func(t *testing.T) {
		p, err := NewWASMParser(t.Context())
		require.NoError(t, err)
		require.NoError(t, p.Close(t.Context()))
	})
}

func TestWASMParser_Parse(t *testing.T) {
	p := newWASMParserForTest(t)

	t.Run("same origin reuse", func(t *testing.T) {
		for range 2 {
			got, err := p.Parse(
				t.Context(),
				ParseInput{Content: "hello", Origin: wasmTestOrigin},
			)
			require.NoError(t, err)
			assert.Contains(t, got.Body, "hello")
		}
	})

	t.Run("different origin", func(t *testing.T) {
		got, err := p.Parse(
			t.Context(),
			ParseInput{Content: "hello", Origin: "https://example.com"},
		)
		require.NoError(t, err)
		assert.Contains(t, got.Body, "hello")
	})
}

func TestWASMParser_ParseQuote(t *testing.T) {
	p := newWASMParserForTest(t)

	t.Run("plain text", func(t *testing.T) {
		got, err := p.ParseQuote(
			t.Context(),
			ParseInput{Content: "hello", Origin: wasmTestOrigin},
		)
		require.NoError(t, err)
		assert.Contains(t, got.Body, "hello")
	})
}

func TestWASMParser_Close(t *testing.T) {
	t.Run("after parse", func(t *testing.T) {
		p, err := NewWASMParser(t.Context())
		require.NoError(t, err)
		_, err = p.Parse(
			t.Context(),
			ParseInput{Content: "hello", Origin: wasmTestOrigin},
		)
		require.NoError(t, err)
		require.NoError(t, p.Close(t.Context()))
	})
}
