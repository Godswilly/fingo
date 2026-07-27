package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Godswilly/fingo/internal/config"
	"github.com/Godswilly/fingo/internal/database/postgres"
	"github.com/Godswilly/fingo/migrations"
)

func main() {
	cfg, err := config.Load(config.RoleMigrate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fingo-migrate config error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fingo-migrate connection error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := migrations.Run(ctx, db.Pool); err != nil {
		fmt.Fprintf(os.Stderr, "fingo-migrate error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("fingo-migrate: migrations applied successfully")
}
