package auth

import (
	"fmt"
	"strings"
	"time"
)

const (
	DefaultOrigin   = "https://q.trap.jp"
	DefaultClientID = ""

	authorizePath = "/api/v3/oauth2/authorize"
	tokenPath     = "/api/v3/oauth2/token"
	revokePath    = "/api/v3/oauth2/revoke"

	callbackPath = "/callback"

	LoginTimeout = 5 * time.Minute
)

func ParseScopes(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t'
	})
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func NormalizeScopes(scopes []string) ([]string, error) {
	seen := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		if s != ScopeRead && s != ScopeWrite {
			return nil, fmt.Errorf(
				"unsupported scope: %s (supported: %s, %s)",
				s,
				ScopeRead,
				ScopeWrite,
			)
		}
		seen[s] = true
	}
	if seen[ScopeWrite] {
		return []string{ScopeRead, ScopeWrite}, nil
	}
	if seen[ScopeRead] {
		return []string{ScopeRead}, nil
	}
	return []string{ScopeRead}, nil
}
