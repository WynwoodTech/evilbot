package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/nlopes/slack"
)

func ParseCommand(cmd string) (string, []string, bool) {
	allparams := strings.Split(cmd, " ")
	params := []string{}
	var cmdString string
	for i, param := range allparams {
		if i == 0 {
			cmdReg := regexp.MustCompile("^!(.*)$")
			if str := cmdReg.FindString(param); len(str) > 0 {
				cmdString = param
				continue
			} else {
				return "", []string{}, false
			}
		}
		params = append(params, param)
	}
	return cmdString, params, true
}

func GetRank(u *slack.User) (int, bool) {
	var score int
	err := db.Batch(func(tx *bolt.Tx) error {
		rb := tx.Bucket([]byte("user_rank"))
		r := rb.Get([]byte(u.ID))
		if r == nil {
			return errors.New("no value")
		}
		dec := gob.NewDecoder(bytes.NewReader(r))
		err := dec.Decode(&score)
		if err != nil {
			log.Printf("LogScore Encode Error: %v\n", err.Error())
			return err
		}
		return nil
	})
	if err != nil {
		return score, false
	}
	return score, true
}

func TopFive() {
	db.Batch(func(tx *bolt.Tx) error {
		rb := tx.Bucket([]byte("user_rank"))
		c := rb.Cursor()
		tops := make([]map[int]string, 5)
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var score int
			dec := gob.NewDecoder(bytes.NewReader(v))
			if err := dec.Decode(&score); err != nil {
				for i, t := range tops {
					for s, u := range t {
						if score >= s {
							log.Printf(" %v:%v\n", u, i)
						}
					}
				}
			}
		}
		return nil
	})

}

func LogScore(u *slack.User, i int) {
	db.Batch(func(tx *bolt.Tx) error {
		rb := tx.Bucket([]byte("user_rank"))
		r := rb.Get([]byte(u.ID))
		dec := gob.NewDecoder(bytes.NewReader(r))
		var score int
		err := dec.Decode(&score)
		if err != nil {
			log.Printf("LogScore Encode Error: %v\n", err.Error())
		}
		score += i
		if score == 1 {
			phrases := []string{
				"Wynwood-Tech",
				"the best internet group since sliced bread",
				"the dark abyss",
				"Jamrock",
				"the interwebz",
				"the beginning of the rest of your life",
			}

			msgParams := slack.NewPostMessageParameters()
			msgParams.AsUser = true
			msgParams.LinkNames = 1
			msgString := fmt.Sprintf(
				"Hey @%v, welcome to %s! Make sure you update your avatar.",
				u.Name,
				phrases[rand.Intn(len(phrases))],
			)
			rtm.PostMessage("#general", msgString, msgParams)
			msgString = "I am Evil Bot here to serve and entertain. My commands are largely Easter-eggs... but you can always ask for !showrank."
			rtm.PostMessage("#general", msgString, msgParams)
		}
		log.Printf("%v's score is now: %v\n\n", u.Name, score)
		var ret bytes.Buffer
		enc := gob.NewEncoder(&ret)
		err = enc.Encode(score)
		if err != nil {
			log.Printf("Encoder Error: %v\n", err)
		}
		rb.Put([]byte(u.ID), ret.Bytes())
		return nil
	})
}

func GiveRank(u *slack.User) bool {
	err := db.Batch(func(tx *bolt.Tx) error {
		gb := tx.Bucket([]byte("rank_giver"))

		g := gb.Get([]byte(u.ID))
		if g == nil {
			var ret bytes.Buffer
			enc := gob.NewEncoder(&ret)
			err := enc.Encode(time.Now())
			if err != nil {
				log.Printf("Encoder Error: %v\n", err)
			}
			gb.Put([]byte(u.ID), ret.Bytes())
			return nil
		}

		dec := gob.NewDecoder(bytes.NewReader(g))
		var lastGave time.Time
		err := dec.Decode(&lastGave)
		if err != nil {
			log.Printf("GiveRank Decode Error: %v\n\n", err.Error())
		}
		if ok := lastGave.Before(time.Now().Add(-(24 * time.Hour))); ok {
			var ret bytes.Buffer
			enc := gob.NewEncoder(&ret)
			err := enc.Encode(time.Now())
			if err != nil {
				log.Printf("Encoder Error: %v\n", err)
			}
			gb.Put([]byte(u.ID), ret.Bytes())
			return nil
		}
		return errors.New("not")
	})
	if err != nil {
		return false
	}
	return true
}

func ParseEvent(ev *slack.MessageEvent) {
	msgParams := slack.NewPostMessageParameters()
	msgParams.AsUser = true
	msgParams.LinkNames = 1
	user, err := api.GetUserInfo(ev.User)
	if err != nil {
		log.Printf("Error: %v", err.Error())
		return
	}
	LogScore(user, 1)
	if cmd, params, ok := ParseCommand(ev.Text); ok {
		log.Printf("%v issued the %v command with %#v params\n\n", user.Name, cmd, params)
		if cmd == "!speak" {
			cleanparams := []string{}
			for _, param := range params {
				nickReg := regexp.MustCompile("<@(.*)>")
				if str := nickReg.FindString(param); len(str) > 0 {
					nick, err := api.GetUserInfo(str[2 : len(str)-1])
					if err != nil {
						continue
					}
					mention := "@" + nick.Name
					cleanparams = append(cleanparams, mention)
					continue
				}
				cleanparams = append(cleanparams, param)
				continue
			}

			msgString := strings.Join(cleanparams, " ")
			rtm.PostMessage("#general", msgString, msgParams)
			return
		} else if cmd == "!top5" {

		} else if cmd == "!showrank" {
			if len(params) == 0 {
				score, ok := GetRank(user)
				if ok {
					msgString := fmt.Sprintf("%v's score is: %v", user.Name, score)
					rtm.PostMessage("#general", msgString, msgParams)
				}
			} else if len(params) == 1 {
				nickReg := regexp.MustCompile("<@(.*)>")
				if str := nickReg.FindString(params[0]); len(str) > 0 {
					nick, err := api.GetUserInfo(str[2 : len(str)-1])
					if err != nil {
						msgString := fmt.Sprintf(
							"%v: The user %v doesn't seem to exist.",
							user.Name,
							params[0],
						)
						rtm.PostMessage("#general", msgString, msgParams)
						return
					}
					score, ok := GetRank(nick)
					if ok {
						msgString := fmt.Sprintf("%v's score is: %v", nick.Name, score)
						rtm.PostMessage("#general", msgString, msgParams)
					}
				}
			} else {
				msgString := fmt.Sprintf("%v: invalid arguments", user.Name)
				rtm.PostMessage("#general", msgString, msgParams)
				msgString = fmt.Sprint("usage: !showrank [username]")
				rtm.PostMessage("#general", msgString, msgParams)
				return
			}
		} else if cmd == "!giverank" {
			if len(params) != 1 {
				msgString := fmt.Sprintf("%v: invalid arguments", user.Name)
				rtm.PostMessage("#general", msgString, msgParams)
				msgString = fmt.Sprint("usage: !giverank <username>")
				rtm.PostMessage("#general", msgString, msgParams)
				return
			}
			nickReg := regexp.MustCompile("<@(.*)>")
			if str := nickReg.FindString(params[0]); len(str) > 0 {
				nickTest := strings.Split(str[2:len(str)-1], "|")
				nick, err := api.GetUserInfo(nickTest[0])
				if err != nil {
					msgString := fmt.Sprintf(
						"%v: The user %v doesn't seem to exist.",
						user.Name,
						params[0],
					)
					rtm.PostMessage("#general", msgString, msgParams)
					return
				}
				if user.Name == nick.Name {
					msgString := fmt.Sprintf(
						"%v: Can't give yourself rank, buddy!",
						user.Name,
					)
					rtm.PostMessage("#general", msgString, msgParams)
					return
				}
				if GiveRank(user) {
					LogScore(nick, 100)
					return
				} else {
					msgString := fmt.Sprintf("Rank given to %v", nick.Name)
					rtm.PostMessage("#general", msgString, msgParams)
				}
			}
		} else {

		}
	}

}

var api *slack.Client
var rtm *slack.RTM
var db *bolt.DB

func tellJokes() {
	jfile, err := os.Open("jokes.json")
	if err != nil {
		log.Panic(err)
	}
	msgParams := slack.NewPostMessageParameters()
	msgParams.AsUser = true
	msgParams.LinkNames = 1

	dec := json.NewDecoder(jfile)
	var jokes map[string][]interface{}
	dec.Decode(&jokes)
	for {
		var pause time.Duration
		for {
			rTime := rand.Intn(540)
			if rTime > 240 {
				pause = time.Duration(rTime) * time.Minute
				break
			}
		}
		time.Sleep(pause)
		c := rand.Intn(len(jokes) - 1)
		i := 0
		for _, js := range jokes {
			if c == i {
				j := rand.Intn(len(js) - 1)
				for jc, jString := range js {
					if jc == j {
						if msgString, ok := jString.(string); ok {
							rtm.PostMessage("#general", msgString, msgParams)
						}
					}
				}
			}
			i++
		}
	}
}

func main() {
	slackbot := os.Getenv("SLACKBOT")
	if len(slackbot) < 1 {
		log.Print("Set ENV variable SLACKBOT to your slackbot API Key")
		return
	}
	var err error
	db, err = bolt.Open("slackbot.db", 0660, nil)
	if err != nil {
		log.Print("DB Error")
		return
	}
	if err := db.Batch(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("user_rank"))
		tx.CreateBucketIfNotExists([]byte("rank_giver"))
		return nil
	}); err != nil {
		log.Panic(err)
	}
	api = slack.New(slackbot)
	_, err = api.AuthTest()
	if err != nil {
		log.Fatal(err)
	}
	rtm = api.NewRTM()
	//params := slack.NewPostMessageParameters()
	//api.PostMessage("#general", "taking over for the 99 and the 2000", params)
	go rtm.ManageConnection()
	go tellJokes()
Loop:
	for {
		select {
		case msg := <-rtm.IncomingEvents:
			//fmt.Print("Event Received: ")
			switch ev := msg.Data.(type) {
			case *slack.HelloEvent:
				// Ignore hello

			case *slack.ConnectedEvent:
				//fmt.Println("Infos:", ev.Info)
				//fmt.Println("Connection counter:", ev.ConnectionCount)
				// Replace #general with your Channel ID
				//rtm.SendMessage(rtm.NewOutgoingMessage("Hello world", "#general"))

			case *slack.MessageEvent:
				//log.Printf("Message: %v\n", ev)
				ParseEvent(ev)

			case *slack.PresenceChangeEvent:
				//log.Printf("Presence Change: %v\n", ev)

			case *slack.LatencyReport:
				//log.Printf("Current latency: %v\n", ev.Value)

			case *slack.RTMError:
				//log.Printf("Error: %s\n", ev.Error())

			case *slack.InvalidAuthEvent:
				log.Printf("Invalid credentials")
				break Loop

			default:

				// gnore other events..
				//log.Printf("Unexpected: %v\n", msg.Data)
			}
		}
	}
	//log.Printf("AT: %v\n", at)
	//chans, err := api.GetChannels(true)
	//if err != nil {
	//log.Fatal(err)
	//}
	//log.Println("Channels")
	//for _, channel := range chans {
	//log.Printf("\t%v", channel)
	//}
	// If you set debugging, it will log all requests to the console
	// Useful when encountering issues
	// api.SetDebug(true)
	//groups, err := api.GetGroups(false)
	//if err != nil {
	//fmt.Printf("%s\n", err)
	//return
	//}
	//for _, group := range groups {
	//fmt.Printf("ID: %s, Name: %s\n", group.ID, group.Name)
	//}
}
