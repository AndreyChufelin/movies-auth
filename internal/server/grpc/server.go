package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/AndreyChufelin/movies-api/pkg/validator"
	"github.com/AndreyChufelin/movies-auth/internal/storage"
	pbuser "github.com/AndreyChufelin/movies-auth/pkg/pb/user"
	"google.golang.org/grpc"
)

type Server struct {
	pbuser.UnimplementedUserServiceServer
	logger    *slog.Logger
	server    *grpc.Server
	port      string
	storage   Storage
	validator *validator.Validator
}

type Storage interface {
	InsertUser(user *storage.User) error
	GetUserByEmail(email string) (*storage.User, error)
	UpdateUser(user *storage.User) error
}

func NewGRPC(logger *slog.Logger, storage Storage, port string) *Server {
	grpcServer := grpc.NewServer()
	return &Server{
		logger:  logger,
		storage: storage,
		server:  grpcServer,
		port:    port,
	}
}

func (s *Server) Start() error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		return fmt.Errorf("failed start tcp server: %w", err)
	}

	s.validator, err = validator.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	s.logger.Info("grpc server started", slog.String("addr", l.Addr().String()))
	pbuser.RegisterUserServiceServer(s.server, s)

	if err := s.server.Serve(l); err != nil {
		return fmt.Errorf("failed to start grpc server: %w", err)
	}

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping grpc server")
	done := make(chan struct{})

	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("grpc server stopped gracefully")
		return nil
	case <-ctx.Done():
		s.logger.Warn("context done, forcing server stop")
		s.server.Stop()
		return fmt.Errorf("stop operation canceled: %w", ctx.Err())
	}
}
