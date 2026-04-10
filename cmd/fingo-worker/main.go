package main

import (
	"fmt"
	"os"

	"github.com/Godswilly/fingo/internal/config"
)

func main() {
	_, err := config.Load(config.RoleWorker)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fingo-worker config error: %v\n", err)
		os.Exit(1)
	}

	// Entrypoint for FinGo async workers (outbox, consumers, schedulers).
}
