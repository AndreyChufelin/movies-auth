package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AndreyChufelin/movies-api/pkg/validator"
	"github.com/AndreyChufelin/movies-auth/internal/storage"
	pbuser "github.com/AndreyChufelin/movies-auth/pkg/pb/user"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
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
	var vErr *validator.ValidationErrors
	if errors.As(err, &vErr) {
		st := status.New(codes.InvalidArgument, "validation error")

		br := &errdetails.BadRequest{}
		for _, e := range vErr.Errors {
			br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field:       e.Field,
				Description: e.Message,
			})
		}

		st, err := st.WithDetails(br)
		if err != nil {
			panic(fmt.Sprintf("Unexpected error attaching metadata: %v", err))
		}

		logg.Warn("validation error", "error", vErr.Error())
		return nil, st.Err()
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

	return &pbuser.UserMessage{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Activated: user.Activated,
		CreatedAt: user.CreatedAt.Unix(),
	}, nil
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

	return &pbuser.UserMessage{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Activated: user.Activated,
		CreatedAt: user.CreatedAt.Unix(),
	}, nil
}
