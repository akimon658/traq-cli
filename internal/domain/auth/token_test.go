package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseScopes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"read", "read", []string{"read"}},
		{"space", "read write", []string{"read", "write"}},
		{"comma", "read,write", []string{"read", "write"}},
		{"mixed", "read , write", []string{"read", "write"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ParseScopes(tt.in))
		})
	}
}

func TestNormalizeScopes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"empty defaults to read", nil, []string{"read"}, false},
		{"read only", []string{"read"}, []string{"read"}, false},
		{"write implies read", []string{"write"}, []string{"read", "write"}, false},
		{"read and write", []string{"read", "write"}, []string{"read", "write"}, false},
		{"unknown scope", []string{"manage_bot"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeScopes(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLacksScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		token Token
		scope string
		want  bool
	}{
		{
			name:  "oauth with scope",
			token: Token{Method: MethodOAuth, AccessToken: "x", Scope: []string{"read"}},
			scope: ScopeRead,
			want:  false,
		},
		{
			name:  "oauth without scope",
			token: Token{Method: MethodOAuth, AccessToken: "x", Scope: []string{"read"}},
			scope: ScopeWrite,
			want:  true,
		},
		{
			name:  "oauth with unknown scopes",
			token: Token{Method: MethodOAuth, AccessToken: "x"},
			scope: ScopeWrite,
			want:  false,
		},
		{
			name:  "bot token",
			token: Token{Method: MethodBot, AccessToken: "x"},
			scope: ScopeWrite,
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.token.LacksScope(tt.scope))
		})
	}
}

func TestTokenExpired(t *testing.T) {
	t.Parallel()
	assert.True(t, (&Token{ExpiresAt: time.Now().Add(-time.Hour)}).Expired(),
		"expired token should be expired")
	assert.False(t, (&Token{ExpiresAt: time.Now().Add(time.Hour)}).Expired(),
		"unexpired token should not be expired")
	assert.False(t, (&Token{}).Expired(),
		"token without expiry should not be expired")
}

func TestContainsScope(t *testing.T) {
	t.Parallel()
	assert.True(t, ContainsScope([]string{"read", "write"}, ScopeWrite),
		"should contain write")
	assert.False(t, ContainsScope([]string{"read"}, ScopeWrite),
		"should not contain write")
}
