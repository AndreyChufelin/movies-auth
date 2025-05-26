package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AndreyChufelin/movies-auth/internal/config"
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

	<-ctx.Done()
	storage.Close(ctx)
}
