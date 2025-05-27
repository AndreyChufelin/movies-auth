package grpcserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/AndreyChufelin/movies-api/pkg/validator"
	"github.com/AndreyChufelin/movies-auth/internal/storage"
	pbuser "github.com/AndreyChufelin/movies-auth/pkg/pb/user"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) Register(ctx context.Context, request *pbuser.RegisterRequest) (*pbuser.RegisterResponse, error) {
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
		logg.Error("failed to insert new user", "email", user.Email)
		return nil, status.Error(codes.Internal, "internal error")
	}

	s.background(func() {
		err = s.mailer.Send(user.Email, "user_welcome.tmpl", user)
		if err != nil {
			logg.Error("failed to send email", "error", err)
		}
	})

	return &pbuser.RegisterResponse{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Activated: user.Activated,
		CreatedAt: user.CreatedAt.Unix(),
	}, nil
}
