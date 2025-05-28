package postgres

import (
	"context"
	"time"

	"github.com/AndreyChufelin/movies-auth/internal/storage"
	"github.com/jackc/pgx/v5"
)

func (s Storage) NewToken(userID int64, ttl time.Duration, scope string) (*storage.Token, error) {
	token, err := storage.GenerateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}
	err = s.InsertToken(token)
	return token, err
}

func (s Storage) InsertToken(token *storage.Token) error {
	query := `
		INSERT INTO tokens (hash, user_id, expiry, scope)
		VALUES (@hash, @user_id, @expiry, @scope)`

	args := pgx.NamedArgs{
		"hash":    token.Hash,
		"user_id": token.UserID,
		"expiry":  token.Expiry,
		"scope":   token.Scope,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s.db.Exec(ctx, query, args)

	return err
}

func (s Storage) DeleteToAllTokensForUser(scope string, userID int64) error {
	query := `
		DELETE FROM tokens
		WHERE scope = @scope AND user_id = @user_id`

	args := pgx.NamedArgs{
		"user_id": userID,
		"scope":   scope,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s.db.Exec(ctx, query, args)

	return err
}
