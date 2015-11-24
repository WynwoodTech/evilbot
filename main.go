package main

import (
	"github.com/nlopes/slack"
	"log"
	"os"
	"regexp"
	"strings"
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

func ParseEvent(ev *slack.MessageEvent) {
	user, err := api.GetUserInfo(ev.User)
	if err != nil {
		log.Printf("Error: %v", err.Error())
		return
	}
	if cmd, params, ok := ParseCommand(ev.Text); ok {
		msgParams := slack.NewPostMessageParameters()
		msgParams.AsUser = true
		log.Printf("%v issued the %v command with %#v params\n\n", user.Name, cmd, params)
		if cmd == "!speak" {
			msgString := strings.Join(params, " ")
			rtm.PostMessage("#general", msgString, msgParams)
		} else {

		}
	}

}

var api *slack.Client
var rtm *slack.RTM

func main() {
	slackbot := os.Getenv("SLACKBOT")
	if len(slackbot) < 1 {
		log.Print("Set ENV variable SLACKBOT to your slackbot API Key")
		return
	}
	api = slack.New(slackbot)
	var err error
	_, err = api.AuthTest()
	if err != nil {
		log.Fatal(err)
	}
	rtm = api.NewRTM()
	//params := slack.NewPostMessageParameters()
	//api.PostMessage("#general", "taking over for the 99 and the 2000", params)
	go rtm.ManageConnection()
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
				rtm.SendMessage(rtm.NewOutgoingMessage("Hello world", "#general"))

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
