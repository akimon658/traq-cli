package auth

//go:generate go tool mockgen -destination=mock_store.go -package=auth github.com/akimon658/traq-cli/internal/domain/auth Store

type Store interface {
	Save(t Token) (string, error)
	Load() (Token, string, bool, error)
	Delete() (bool, error)
}
