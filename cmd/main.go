package main

import (
	"log"
	"os"

	"github.com/wynwoodtech/evilbot/pkg/bot"
)

func main() {
	defer log.Println("Exiting...")
	//Get the Slack API key from your environment variable

	log.Println("Provisioning bot...")
	api_key := os.Getenv("SLACKBOT")
	//Start a New Bot given the API Key and a command indntifier
	//Command identifier cannot be longer than 3 charachters
	b, err := evilbot.New(api_key, ".")
	if err != nil {
		log.Println("Not Authenticated, please check your ENV Setting")
		return
	}

	//Add Command Handlers, you can also create an array of handlers and loop trhough them
	b.AddCmdHandler(
		"test",
		TestCmdHandler,
	)
	b.AddCmdHandler(
		"help",
		TestCmdHandler2,
	)
	//Adding general purpose event handlers. These can be used for anything other than a command
	//example would be keeping track of when a person was last active
	b.AddEventHandler(
		"event",
		TestGeneralHandler,
	)

	//Turn on logging to see ALL Incoming data
	b.Logging(true)

	//Run The Bot
	b.Run()
}

//These are just some example handlers

func TestCmdHandler(e bot.Event, r *evilbot.Response) {
	log.Printf("Test Command: %v\n", e)
}

func TestCmdHandler2(e bot.Event, r *evilbot.Response) {
	log.Printf("Test Command2: %v\n", e)
}

func TestGeneralHandler(e bot.Event, r *evilbot.Response) {
	log.Printf("Test General Handler: %v\n", e)
}
