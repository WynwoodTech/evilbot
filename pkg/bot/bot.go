package bot

import (
	"log"
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
	Name, Channel, User, ArgStr string
	Time                        time.Time
}

type Response struct {
	RTM *slack.RTMResponse
}

func (s *SlackBot) HandleMsg(e *slack.MessageEvent) {
	log.Printf("MSG: %#v\n", e)
	for _, evh := range s.handlers {
		log.Printf("EVH: %#v\n", evh)
		ev := Event{}
		log.Printf("EV: %#v\n", ev)
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

func New(apiKey string) (*SlackBot, error) {

	api := slack.New(apiKey)
	rtm := api.NewRTM()
	if _, err := rtm.AuthTest(); err != nil {
		return nil, err
	}

	newBot := SlackBot{}

	newBot.name = apiKey
	newBot.conn = api
	newBot.rtm = rtm

	log.Printf("NewBot: %#v\n", newBot)
	return &newBot, nil
}
