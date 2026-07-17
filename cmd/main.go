package main

import (
	"fmt"
	"log"

	"github.com/klemanjar0/go-notifier-bot/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Hello")
	_ = cfg
}
