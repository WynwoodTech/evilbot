package main

import (
	"log"
	"net/http"
	"os"

	"github.com/wynwoodtech/evilbot/pkg/bot"
	"github.com/wynwoodtech/evilbot/pkg/storage"
)

func main() {
	defer log.Println("Exiting...")

	//Get the Slack API key from your environment variable
	log.Println("Provisioning bot...")
	api_key := os.Getenv("SLACKBOT")
	if len(api_key) < 1 {
		log.Printf("Please et proper SLACKBOT environment variable")
		return
	}

	//Load storage given slackbot key
	log.Printf("Loading storage volumes...")
	db, err := storage.Load(api_key)
	if err != nil {
		panic(err)
	}
	log.Printf("DB Loaded: %v\n", db)
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

	if err := db.LoadBucket("main"); err != nil {
		log.Panic(err)
	}

	if err := db.SetVal("main", "test2", "Test Value22"); err != nil {
		log.Panic(err)
	}

	if t, err := db.GetVal("main", "test2"); err == nil {
		log.Printf("Test Key: %v\n", t)
	} else {
		log.Printf("Reading Error: %v\n", err)
	}

	if t, err := db.GetVal("main", "test"); err == nil {
		log.Printf("Test Key: %v\n", t)
	} else {
		log.Printf("Reading Error: %v\n", err)
	}
	//Turn on logging to see ALL Incoming data
	b.Logging(true)

	//Register Additional Endpoints
	if err := b.RegisterEndpoint("/test", "get", TestHTTPHandler); err != nil {
		log.Printf("Endpoint Error: %v\n", err)
	}

	if err := b.AcitivityLogger(); err != nil {
		log.Panic(err)
	}

	//Run The Bot
	b.RunWithHTTP("8000")
}

//These are just some example bot handlers
func TestCmdHandler(e evilbot.Event, r *evilbot.Response) {
	r.ReplyToUser(&e, "test")
	log.Printf("Test Command: %v\n", e)
}

func TestCmdHandler2(e evilbot.Event, r *evilbot.Response) {
	log.Printf("Test Command2: %v\n", e)
}

func TestGeneralHandler(e evilbot.Event, r *evilbot.Response) {
	log.Printf("Test General Handler: %v\n", e)
}

//These are just some example http bot handlers
func TestHTTPHandler(rw http.ResponseWriter, r *http.Request, br *evilbot.Response) {
	rw.Write([]byte("test"))
}
