package message

import (
	"fmt"

	"uuid"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// ValidateListOptions は list フラグ値を認証前に検証する。
// 認証エラーより使い方の誤りを優先するため、cli は ResolveToken の前に呼ぶこと。
func ValidateListOptions(opts ListOptions) error {
	if opts.Limit < 1 {
		return exit.Usagef("--limit must be at least 1")
	}
	if opts.Offset < 0 {
		return exit.Usagef("--offset must not be negative")
	}
	if opts.Order != "asc" && opts.Order != "desc" {
		return exit.Usagef(`--order must be "asc" or "desc"`)
	}
	if _, err := ParseRFC3339Flag("since", opts.Since); err != nil {
		return err
	}
	if _, err := ParseRFC3339Flag("until", opts.Until); err != nil {
		return err
	}
	return nil
}

// ValidateSearchOptions は search フラグ値を認証前に検証する。
func ValidateSearchOptions(word string, opts SearchOptions) error {
	_ = word
	if opts.Limit < 1 || opts.Limit > MaxSearchLimit {
		return exit.Usagef("--limit must be between 1 and %d", MaxSearchLimit)
	}
	if opts.Offset < 0 || opts.Offset > MaxSearchOffset {
		return exit.Usagef("--offset must be between 0 and %d", MaxSearchOffset)
	}
	switch opts.Sort {
	case "createdAt", "-createdAt", "updatedAt", "-updatedAt":
	default:
		return exit.UsageError(
			fmt.Errorf(
				`--sort must be one of "createdAt", "-createdAt", "updatedAt", "-updatedAt"`,
			),
		)
	}
	if _, err := ParseRFC3339Flag("after", opts.After); err != nil {
		return err
	}
	if _, err := ParseRFC3339Flag("before", opts.Before); err != nil {
		return err
	}
	if opts.Citation != "" {
		if _, err := uuid.Parse(opts.Citation); err != nil {
			return exit.Usagef("invalid --citation: %v", err)
		}
	}
	return nil
}
