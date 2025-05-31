package grpcserver

import (
	"context"
	"errors"
	"time"

	"github.com/AndreyChufelin/movies-auth/internal/storage"
	pbuser "github.com/AndreyChufelin/movies-auth/pkg/pb/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) Register(ctx context.Context, request *pbuser.RegisterRequest) (*pbuser.UserMessage, error) {
	logg := s.logger.With("handler", "register user")
	logg.Info("REQUEST")

	user := &storage.User{
		Email:     request.Email,
		Name:      request.Name,
		Activated: false,
	}
	if request.Password != "" {
		user.SetPassword(request.Password)
	}

	err := s.validator.Validate(user)
	if err != nil {
		return nil, validationError(logg, err)
	}

	err = s.storage.InsertUser(user)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateEmail) {
			logg.Warn("email already exists", "email", user.Email)
			return nil, status.Error(codes.AlreadyExists, "email already exists")
		}
		logg.Error("failed to insert new user", "email", user.Email, "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	token, err := s.storage.NewToken(user.ID, 3*24*time.Hour, storage.ScopeActivation)
	if err != nil {
		logg.Error("failed to generate new token", "user_id", user.ID, "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	s.background(func() {
		data := map[string]interface{}{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}

		err = s.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			logg.Error("failed to send email", "error", err)
		}
	})

	return userToUserMessage(user), nil
}

func (s *Server) Activated(ctx context.Context, request *pbuser.ActivatedRequest) (*pbuser.UserMessage, error) {
	logg := s.logger.With("handler", "activated")
	logg.Info("REQUEST")

	if len(request.Token) != 26 {
		return nil, status.Error(codes.InvalidArgument, "invalid token")
	}

	user, err := s.storage.GetUserForToken(storage.ScopeActivation, request.Token)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			logg.Warn("invalid or expired token")
			return nil, status.Error(codes.InvalidArgument, "invalid or expired token")
		}
		logg.Error("failed to get user for token", "error", err)
		return nil, status.Error(codes.Internal, "activation failed")
	}

	user.Activated = true

	err = s.storage.UpdateUser(user)
	if err != nil {
		if errors.Is(err, storage.ErrEditConflict) {
			logg.Warn("edit conflict")
			return nil, status.Error(codes.Internal, "edit conflict")
		}
		logg.Error("failed to update user", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	err = s.storage.DeleteToAllTokensForUser(storage.ScopeActivation, user.ID)
	if err != nil {
		logg.Error("failed to delete tokens for user", "user_id", user.ID, "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return userToUserMessage(user), nil
}

func (s *Server) Authentication(ctx context.Context, request *pbuser.AuthenticationRequest) (*pbuser.AuthenticationResponse, error) {
	logg := s.logger.With("handler", "authentication")
	logg.Info("REQUEST")

	input := struct {
		Email    string `validate:"required,email"`
		Password string `validate:"required,gte=8,lte=72"`
	}{request.Email, request.Password}

	err := s.validator.Validate(input)
	if err != nil {
		return nil, validationError(logg, err)
	}

	user, err := s.storage.GetUserByEmail(request.Email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			logg.Warn("user doesn't exist")
			return nil, status.Error(codes.InvalidArgument, "user not exist")
		}
		logg.Error("failed to get user by email", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	match, err := user.PasswordMatches(request.Password)
	if err != nil {
		logg.Error("failed to match password", "id", user.ID, "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	if !match {
		logg.Warn("invalid password", "id", user.ID)
		return nil, status.Error(codes.InvalidArgument, "invalid password")
	}

	token, err := s.storage.NewToken(user.ID, 24*time.Hour, storage.ScopeAuthentication)
	if err != nil {
		logg.Error("failed te create new token", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &pbuser.AuthenticationResponse{
		Token:  token.Plaintext,
		Expiry: token.Expiry.Unix(),
	}, nil
}

func (s *Server) VerifyToken(ctx context.Context, request *pbuser.VerifyTokenRequest) (*pbuser.UserMessage, error) {
	logg := s.logger.With("handler", "verify token")
	logg.Info("REQUEST")

	if request.Token == "" {
		user := storage.AnonymousUser
		return userToUserMessage(user), nil
	}

	if len(request.Token) != 26 {
		return nil, status.Error(codes.InvalidArgument, "invalid token")
	}

	user, err := s.storage.GetUserForToken(storage.ScopeAuthentication, request.Token)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		logg.Error("failed to get user by token", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	user.Permissions, err = s.storage.GetAllUserPermissions(user.ID)
	if err != nil {
		logg.Error("failed to get user permissions", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return userToUserMessage(user), nil
}

func userToUserMessage(user *storage.User) *pbuser.UserMessage {
	return &pbuser.UserMessage{
		Id:          user.ID,
		Name:        user.Name,
		Email:       user.Email,
		Activated:   user.Activated,
		CreatedAt:   user.CreatedAt.Unix(),
		Permissions: user.Permissions,
	}
}
