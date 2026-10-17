package auth

import (
	"reflect"
	"testing"
	"time"
)

func TestParseScopes(t *testing.T) {
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
			got := ParseScopes(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseScopes(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeScopes(t *testing.T) {
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
			got, err := NormalizeScopes(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeScopes(%v) error = %v, wantErr %v", tt.in, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeScopes(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestLacksScope(t *testing.T) {
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
			if got := tt.token.LacksScope(tt.scope); got != tt.want {
				t.Errorf("LacksScope(%q) = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestTokenExpired(t *testing.T) {
	if (&Token{ExpiresAt: time.Now().Add(-time.Hour)}).Expired() != true {
		t.Error("expired token should be expired")
	}
	if (&Token{ExpiresAt: time.Now().Add(time.Hour)}).Expired() != false {
		t.Error("unexpired token should not be expired")
	}
	if (&Token{}).Expired() != false {
		t.Error("token without expiry should not be expired")
	}
}

func TestContainsScope(t *testing.T) {
	if !ContainsScope([]string{"read", "write"}, ScopeWrite) {
		t.Error("should contain write")
	}
	if ContainsScope([]string{"read"}, ScopeWrite) {
		t.Error("should not contain write")
	}
}
