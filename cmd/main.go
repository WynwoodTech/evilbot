package main

import (
	"log"
	"os"

	"github.com/lirancohen/slackbot/pkg/bot"
)

func main() {

	api_key := os.Getenv("SLACKBOT")
	b, err := bot.New(api_key, ".")
	if err != nil {
		log.Println("Not Authenticated, please check your ENV Setting")
		return
	}

	b.AddCmdHandler(
		"test",
		TestCmdHandler,
	)

	b.AddCmdHandler(
		"help",
		TestCmdHandler2,
	)

	b.AddEventHandler(
		"event",
		TestGeneralHandler,
	)
	b.Logging(true)
	b.Run()
	log.Println("Exiting...")
}

func TestCmdHandler(e bot.Event, r *bot.Response) {
	log.Printf("Test Command: %v\n", e)
}

func TestCmdHandler2(e bot.Event, r *bot.Response) {
	log.Printf("Test Command2: %v\n", e)
}

func TestGeneralHandler(e bot.Event, r *bot.Response) {
	log.Printf("Test General Handler: %v\n", e)
}
