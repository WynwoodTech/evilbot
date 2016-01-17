package bot

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/nlopes/slack"
)

type SlackBot struct {
	name     string
	leadchar string
	conn     *slack.Client
	rtm      *slack.RTM
	handlers []Handler
}

type Handler interface {
	ServeHandler(e Event, r *Response)
}

type HandlerFunc func(e Event, r *Response)

func (h HandlerFunc) ServeHandler(e Event, r *Response) {
	h(e, r)
}

type Event struct {
	User                      *slack.User
	Channel                   *slack.Channel
	Command, ArgStr, LeadChar string
	Time                      time.Time
}

func (e *Event) ParseCommand(cmd string) error {
	allparams := strings.Split(cmd, " ")
	for i, param := range allparams {
		if i == 0 {
			rgStr := fmt.Sprintf("%v%v%v", "^", e.LeadChar, "(.*)$")
			cmdReg := regexp.MustCompile(rgStr)
			if str := cmdReg.FindString(param); len(str) > 0 {
				e.Command = param[len(e.LeadChar):]
				continue
			} else {
				return errors.New("Unable to parse command")
			}
		}
	}
	e.ArgStr = strings.Join(allparams[1:], " ")
	return nil
}

type Response struct {
	RTM *slack.RTMResponse
}

func (s *SlackBot) HandleMsg(e *slack.MessageEvent) {
	//log.Printf("MSG: %#v\n", e)
	ev := Event{}
	ev.LeadChar = s.leadchar
	if err := ev.ParseCommand(e.Text); err != nil {
		// not a command
	}
	log.Printf("EV: %#v\n", ev)
	for _, evh := range s.handlers {
		log.Printf("EVH: %#v\n", evh)
		ev.User, _ = s.rtm.GetUserInfo(e.User)
		ev.Channel, _ = s.rtm.GetChannelInfo(e.Channel)
	}
}

func (s *SlackBot) HandleConnEvent(e *slack.ConnectedEvent) {
	log.Printf("connection Event: %#v\n", e)
}

func (s *SlackBot) AddEventHandler(name string, ev func(e Event, r *Response)) {
	s.handlers = append(s.handlers)
}

func (s *SlackBot) Run() {
	go s.rtm.ManageConnection()
Loop:
	for {
		select {
		case msg := <-s.rtm.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.HelloEvent:
				// Ignore hello

			case *slack.ConnectedEvent:
				go s.HandleConnEvent(ev)

			case *slack.MessageEvent:
				go s.HandleMsg(ev)

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
}

func New(apiKey string, leadChars string) (*SlackBot, error) {
	if len(leadChars) > 3 {
		return nil, errors.New("Lead Characthers cannot be more than 3 characters long")
	}
	api := slack.New(apiKey)
	rtm := api.NewRTM()
	if _, err := rtm.AuthTest(); err != nil {
		return nil, err
	}

	newBot := SlackBot{}

	newBot.name = apiKey
	newBot.conn = api
	newBot.rtm = rtm
	newBot.leadchar = leadChars

	log.Printf("NewBot: %#v\n", newBot)
	return &newBot, nil
}
