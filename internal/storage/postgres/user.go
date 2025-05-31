package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/AndreyChufelin/movies-auth/internal/storage"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s Storage) InsertUser(user *storage.User) error {
	query := `
		INSERT INTO users (name, email, password_hash, activated)
		VALUES (@name, @email, @password, @activated)
		RETURNING id, created_at, version`

	args := pgx.NamedArgs{
		"name":      user.Name,
		"email":     user.Email,
		"password":  user.PasswordHash,
		"activated": user.Activated,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := s.db.QueryRow(ctx, query, args).
		Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) && e.Code == pgerrcode.UniqueViolation {
			return storage.ErrDuplicateEmail
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

func (s Storage) GetUserByEmail(email string) (*storage.User, error) {
	query := `
		SELECT id, created_at, name, email, password_hash, activated, version
		FROM users
		WHERE email = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	row, err := s.db.Query(ctx, query, email)
	if err != nil {
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}

	user, err := pgx.CollectOneRow(row, pgx.RowToStructByName[storage.User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to collect user by email: %w", err)
	}

	return &user, nil
}

func (s Storage) UpdateUser(user *storage.User) error {
	query := `
		UPDATE users
		SET name = @name, email = @email, password_hash = @password, activated = @activated, version = version + 1
		WHERE id = @id AND version = @version
		RETURNING version`

	args := pgx.NamedArgs{
		"name":      user.Name,
		"email":     user.Email,
		"password":  user.PasswordHash,
		"activated": user.Activated,
		"id":        user.ID,
		"version":   user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := s.db.QueryRow(ctx, query, args).Scan(&user.Version)
	if err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) && e.Code == pgerrcode.UniqueViolation {
			return storage.ErrDuplicateEmail
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return storage.ErrEditConflict
		}
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (s Storage) GetUserForToken(scope, tokenPlaintext string) (*storage.User, error) {
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	query := `
		SELECT users.id, users.created_at, users.name, users.email, users.password_hash, users.activated, users.version
		FROM users
		INNER JOIN tokens
		ON users.id = tokens.user_id
		WHERE tokens.hash = @hash
		AND tokens.scope = @scope
		AND tokens.expiry > @expiry`

	args := pgx.NamedArgs{
		"hash":   tokenHash[:],
		"scope":  scope,
		"expiry": time.Now(),
	}

	var user storage.User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := s.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("failed to query user for token: %w", err)
	}
	user, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[storage.User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to collect user for token: %w", err)
	}

	return &user, nil
}

func (s Storage) GetAllUserPermissions(userID int64) (storage.Permissions, error) {
	query := `
		SELECT permissions.code AS Permissions
		FROM permissions
		INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
		INNER JOIN users ON users_permissions.user_id = users.id
		WHERE users.id = @user_id`

	args := pgx.NamedArgs{
		"user_id": userID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := s.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("failed to query user permissions: %w", err)
	}

	perm, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to collect user permissions: %w", err)
	}
	permissions := storage.Permissions(perm)

	return permissions, nil
}
