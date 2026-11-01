package main

import (
	"log"

	"godot-builder/internal/api"
)

func main() {
	if err := api.Run(); err != nil {
		log.Fatal(err)
	}
}
