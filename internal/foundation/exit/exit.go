package exit

import "fmt"

// 終了コード体系。AIエージェントからの利用を想定し、コードと1行の標準エラーで挙動を示す。
const (
	General           = 1
	Usage             = 2
	AuthRequired      = 3
	ScopeInsufficient = 4
)

// Error は終了コード付きエラー。cli.Execute がコードに変換する。
// session や各ドメインサービスもこの型で返すため、cli に依存しない独立パッケージに置く。
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// New は指定コードの Error を作る。
func New(code int, err error) *Error {
	return &Error{Code: code, Err: err}
}

// UsageError は使い方の誤り (code 2) を作る。
func UsageError(err error) *Error {
	return &Error{Code: Usage, Err: err}
}

// Usagef は整形して使い方の誤りを作る。
func Usagef(format string, args ...any) *Error {
	return &Error{Code: Usage, Err: fmt.Errorf(format, args...)}
}

// GeneralError は実行時エラー (code 1) を作る。
func GeneralError(err error) *Error {
	return &Error{Code: General, Err: err}
}

// Generalf は整形して実行時エラーを作る。
func Generalf(format string, args ...any) *Error {
	return &Error{Code: General, Err: fmt.Errorf(format, args...)}
}

// AuthRequiredError は未認証エラー (code 3) を作る。
func AuthRequiredError(err error) *Error {
	return &Error{Code: AuthRequired, Err: err}
}

// AuthRequiredf は整形して未認証エラーを作る。
func AuthRequiredf(format string, args ...any) *Error {
	return &Error{Code: AuthRequired, Err: fmt.Errorf(format, args...)}
}

// ScopeInsufficientError はスコープ不足エラー (code 4) を作る。
func ScopeInsufficientError(err error) *Error {
	return &Error{Code: ScopeInsufficient, Err: err}
}
