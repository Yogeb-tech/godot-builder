package main

import (
	"fmt"
	"log"

	"godot-builder/internal/builder"
	"godot-builder/internal/config"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	b, err := builder.New(cfg)
	if err != nil {
		log.Fatalf("Builder initialization error: %v", err)
	}

	if err := b.Build(); err != nil {
		log.Fatalf("Build failed: %v", err)
	}

	fmt.Printf("Build completed successfully\n")
	fmt.Printf("Artifacts available in: %s\n", cfg.OutputDir)
}
