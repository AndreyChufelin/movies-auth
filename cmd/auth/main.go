package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AndreyChufelin/movies-auth/internal/config"
	grpcserver "github.com/AndreyChufelin/movies-auth/internal/server/grpc"
	"github.com/AndreyChufelin/movies-auth/internal/storage/postgres"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	logg := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config, err := config.LoadConfig("configs/config-auth.toml")
	if err != nil {
		logg.Error(
			"failed to load config",
			"error", err,
		)
		cancel()
	}

	logg.Info("connecting to database")
	storage := postgres.NewStorage(
		config.DB.Host,
		config.DB.Port,
		config.DB.User,
		config.DB.Password,
		config.DB.Name,
	)
	err = storage.Connect(ctx)
	if err != nil {
		logg.Error(
			"falied to create connection with database",
			"error", err,
		)
		cancel()
	}

	server := grpcserver.NewGRPC(logg, storage, "50051")
	go func() {
		if err := server.Start(); err != nil {
			logg.Error("failed to start grpc server", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()

	ctxStop, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	storage.Close(ctx)

	if err := server.Stop(ctxStop); err != nil {
		logg.Error("failed to stop grpc server", "err", err)
	}
}
