package main

import (
	"bot/service"
	"fmt"
	"os"
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		fmt.Println("TOKEN is not set")
		return
	}
	println("Start!")
	if err := service.StartBot(token); err != nil {
		fmt.Println("Error starting bot:", err)
	}
}
