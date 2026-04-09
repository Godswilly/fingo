package main

import (
	"fmt"
	"os"

	"github.com/Godswilly/fingo/internal/config"
)

func main() {
	_, err := config.Load(config.RoleAPI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fingo-api config error: %v\n", err)
		os.Exit(1)
	}

	// Entrypoint for FinGo API service.
}
