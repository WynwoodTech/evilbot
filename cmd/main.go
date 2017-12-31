package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/wynwoodtech/evilbot/pkg/activitylog"
	"github.com/wynwoodtech/evilbot/pkg/bot"
	"github.com/wynwoodtech/evilbot/pkg/storage"

	"github.com/gorilla/mux"
	"github.com/wynwoodtech/evilbot/pkg/coin"
	"time"
)

func main() {
	defer log.Println("Exiting...")
	go getMarketSummariesEveryMinute()

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
	b, err := evilbot.New(api_key, "!")
	if err != nil {
		log.Println("Not Authenticated, please check your ENV Setting")
		log.Printf("Error: %v\n", err.Error())
		return
	}

	//Add Command Handlers, you can also create an array of handlers and loop trhough them
	b.AddCmdHandler(
		"market",
		MarketCmdHandler,
	)
	b.AddCmdHandler(
		"listmarkets",
		ListMarketCmdHandler,
	)
	b.AddCmdHandler(
		"help",
		TestCmdHandler2,
	)
	b.AddCmdHandler(
		"speak",
		SpeakCmdHandler,
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
	b.SetLogging(true)

	//Register Additional Endpoints
	if err := b.RegisterEndpoint("/test/{channel}", "get", TestHTTPHandler); err != nil {
		log.Printf("Endpoint Error: %v\n", err)
	}

	if err := activitylog.NewLogger(b); err != nil {
		log.Panic(err)
	}

	//Run The Bot
	b.RunWithHTTP("8000")
}

//These are just some example bot handlers
func MarketCmdHandler(e evilbot.Event, r *evilbot.Response) {
	if len(e.ArgStr) > 0 {
		ms := coin.GetCurrency(e.ArgStr)
		if len(ms.MarketName) < 1 {
			r.ReplyToUser(&e, "Market doesn't exist")
			return
		}
		r.SendToChannel(e.Channel.ID, ms.String())
	}
	return
}

func ListMarketCmdHandler(e evilbot.Event, r *evilbot.Response) {
	if len(e.ArgStr) > 0 {
		ml := coin.ListMarkets()

		if len(ml) < 1 {
			r.ReplyToUser(&e, "No Markets :(")
			return
		}
		var rs string
		var n int
		for k, v := range ml {
			if n > 15 {
				r.ReplyToUser(&e, "Too many markets to list here buddy, try not using a baseline currency")
				return
			}
			if strings.Contains(k, e.ArgStr) {
				n++
				markets := strings.Split(k, "-")
				usdMarket := markets[0] + "-USD"
				usdPrice := coin.GetUSDValue(v,usdMarket)
				if len(usdPrice) > 0 {
					rs += fmt.Sprintf("[%s] ~ $%s\n", k,usdPrice)
					continue
				}
				rs += fmt.Sprintf("[%s]\n", k)
			}

		}

		if len(rs) == 0 {
			r.SendToChannel(e.Channel.ID, "I've never heard of that before")
			return
		}
		r.SendToChannel(e.Channel.ID, rs)
		return
	}

	r.SendToChannel(e.Channel.ID, "Too many markets to list bruh, which coin do you want? example: !listmarkets XVG")
	return
}

func TestCmdHandler2(e evilbot.Event, r *evilbot.Response) {
	log.Printf("Test Command2: %v\n", e)
}

func SpeakCmdHandler(e evilbot.Event, r *evilbot.Response) {
	if e.Channel == nil {
		log.Printf("%v is using the speak command", e.User.Name)
		params := strings.Split(e.ArgStr, " ")
		cleanparams := []string{}
		checkNick := regexp.MustCompile("<@(\\w+)>")
		checkChan := regexp.MustCompile("<#(\\w+)>")
		checkPunct := regexp.MustCompile("([,.!?\\/-]+)")
		for _, param := range params {
			userMatch := checkNick.FindSubmatch([]byte(param))
			chanMatch := checkChan.FindSubmatch([]byte(param))
			if len(userMatch) > 1 {
				if user, err := r.RTM.GetUserInfo(string(userMatch[1])); err != nil {
					r.ReplyToUser(&e, "invalid user mentioned")
					return
				} else {
					punctMatch := checkPunct.FindSubmatch([]byte(param))
					var returnNick string
					if len(punctMatch) > 1 {
						returnNick = "@" + user.Name + string(punctMatch[1])
					} else {
						returnNick = "@" + user.Name
					}
					cleanparams = append(cleanparams, returnNick)
				}
			} else if len(chanMatch) > 1 {
				if channel, err := r.RTM.GetChannelInfo(string(chanMatch[1])); err != nil {
					r.ReplyToUser(&e, "invalid channel mention")
					return
				} else {
					punctMatch := checkPunct.FindSubmatch([]byte(param))
					var returnChan string
					if len(punctMatch) > 1 {
						returnChan = "#" + channel.Name + string(punctMatch[1])
					} else {
						returnChan = "@" + channel.Name
					}
					cleanparams = append(cleanparams, returnChan)

				}
			} else {
				cleanparams = append(cleanparams, param)
			}
		}
		if len(cleanparams) > 0 {
			r.SendToChannel("general", strings.Join(cleanparams, " "))
		}
	}
}

func TestGeneralHandler(e evilbot.Event, r *evilbot.Response) {
	log.Printf("Test General Handler: %v\n", e)
}

//These are just some example http bot handlers
func TestHTTPHandler(rw http.ResponseWriter, r *http.Request, br *evilbot.Response) {
	vars := mux.Vars(r)
	cname := vars["channel"]
	var cid string
	log.Printf("Channel: %v\n", cname)
	if chans, err := br.RTM.GetChannels(true); err == nil {
		for _, ch := range chans {
			if ch.Name == cname {
				cid = ch.ID
			}
		}
	}
	if len(cid) > 0 {
		c, err := br.RTM.GetChannelInfo(cid)
		if err != nil {
			rw.Write([]byte(fmt.Sprintf("%v: %v", "channel doesn't exist", err.Error())))
		} else {
			log.Printf("Channel: %#v\n", c)
			rw.Write([]byte("success"))
		}
	} else {
		rw.Write([]byte(fmt.Sprintf("%v: %v", "channel doesn't exist")))
	}
	log.Printf("Vars: %#v\n", vars)
}

func getMarketSummariesEveryMinute() {
	timeout, err := time.ParseDuration("1m")
	if err != nil {
		panic(err)
	}
	for {
		coin.GetMarketSummaryBittrex()
		coin.GetMarketSummaryBinance()
		time.Sleep(timeout)
	}
}
