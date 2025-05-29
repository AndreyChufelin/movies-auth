package storage

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrDuplicateEmail = errors.New("duplicated email")
	ErrUserNotFound   = errors.New("user not found")
	ErrEditConflict   = errors.New("user not found")
)

var AnonymousUser = &User{}

type User struct {
	ID           int64     `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Name         string    `json:"name" validate:"required,lte=500"`
	Email        string    `json:"email" validate:"required,email"`
	PasswordHash []byte    `json:"-"`
	Password     *string   `db:"-" json:"-" validate:"required,gte=8,lte=72"`
	Activated    bool      `json:"activated"`
	Version      int       `json:"-"`
}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

func (u *User) SetPassword(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}
	u.Password = &plaintextPassword
	u.PasswordHash = hash
	return nil
}

func (u *User) PasswordMatches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}
