package main

import (
	"log"
	"os"

	"github.com/lirancohen/slackBot/pkg/bot"
)

func main() {

	api_key := os.Getenv("SLACKBOT")
	b, err := bot.New(api_key, ".")
	if err != nil {
		log.Println("Not Authenticated, please check your ENV Setting")
		return
	}
	b.Run()
	log.Println("Exiting...")
}
