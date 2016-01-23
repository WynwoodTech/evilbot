package evilbot

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/nlopes/slack"
)

func (s *SlackBot) MParams() slack.PostMessageParameters {
	msgParams := slack.NewPostMessageParameters()
	msgParams.AsUser = true
	msgParams.LinkNames = 1
	return msgParams
}

type SlackBot struct {
	name     string
	logging  bool
	leadchar string
	conn     *slack.Client
	rtm      *slack.RTM
	handlers []EventHandler
	commands []EventHandler
	netcon   *NetConn
}

type NetConn struct {
	port   string
	router *mux.Router
}

type EventHandler struct {
	Name   string
	Handle Handler
}

type Handler interface {
	ServeHandler(e Event, r *Response)
}

type HandlerFunc func(e Event, r *Response)

func (h HandlerFunc) ServeHandler(e Event, r *Response) {
	h(e, r)
}

type EPHandlerFunc func(rw http.ResponseWriter, r *http.Request, br *Response)

func (s *SlackBot) Wrap(h EPHandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rb := Response{}
		rb.RTM = s.rtm
		h(rw, r, &rb)
	})
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
			rgStr := fmt.Sprintf("%v%v%v", "^[", e.LeadChar, "](.*)$")
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
	RTM *slack.RTM
}

func (s *SlackBot) HandleEvent(e *slack.MessageEvent) {
	ev := Event{}
	ev.LeadChar = s.leadchar
	ev.User, _ = s.rtm.GetUserInfo(e.User)
	ev.Channel, _ = s.rtm.GetChannelInfo(e.Channel)
	ev.ArgStr = e.Text
	s.HandleMsg(ev)
	if err := ev.ParseCommand(e.Text); err == nil {
		s.HandleCmd(ev)
	}
}

func (s *SlackBot) HandleCmd(e Event) {
	for _, cmd := range s.commands {
		if strings.ToLower(e.Command) == strings.ToLower(cmd.Name) {
			r := Response{}
			r.RTM = s.rtm
			cmd.Handle.ServeHandler(e, &r)
		}
	}
}

func (s *SlackBot) HandleMsg(e Event) {
	for _, ev := range s.handlers {
		r := Response{}
		r.RTM = s.rtm
		ev.Handle.ServeHandler(e, &r)
	}
}

func (s *SlackBot) HandleConnEvent(e *slack.ConnectedEvent) {
	//log.Printf("connection Event: %#v\n", e)
}

func (s *SlackBot) AddCmdHandler(name string, ev HandlerFunc) error {
	for _, c := range s.commands {
		if strings.ToLower(c.Name) == strings.ToLower(name) {
			return errors.New("Command handler with this name already exists")
		}
	}
	e := EventHandler{}
	e.Name = name
	e.Handle = ev
	s.commands = append(s.commands, e)
	return nil
}

func (s *SlackBot) AddEventHandler(name string, ev HandlerFunc) error {
	for _, h := range s.handlers {
		if strings.ToLower(h.Name) == strings.ToLower(name) {
			return errors.New("General handler with this name already exists")
		}
	}
	e := EventHandler{}
	e.Name = name
	e.Handle = ev
	s.handlers = append(s.handlers, e)
	return nil
}

func (s *SlackBot) Logging(b bool) {
	s.logging = b
}
func (s *SlackBot) Run() {
	s.run()
}
func (s *SlackBot) RunWithHTTP(port string) {

	go http.ListenAndServe(fmt.Sprintf(":%v", port), s.netcon.router)
	s.run()
}
func (s *SlackBot) run() {

	go s.rtm.ManageConnection()
Loop:
	for {
		select {
		case msg := <-s.rtm.IncomingEvents:
			if s.logging {
				log.Printf("Logger: %#v\n", msg.Data)
			}
			switch ev := msg.Data.(type) {
			case *slack.HelloEvent:
				// Ignore hello

			case *slack.ConnectedEvent:
				//go s.HandleConnEvent(ev)

			case *slack.MessageEvent:
				go s.HandleEvent(ev)

			case *slack.PresenceChangeEvent:
				//log.Printf("Presence Change: %v\n", ev)

			case *slack.LatencyReport:
				//log.Printf("Current latency: %v\n", ev.Value)

			case *slack.RTMError:
				//log.Printf("Error: %s\n", ev.Error())

			case *slack.InvalidAuthEvent:
				//log.Printf("Invalid credentials")
				break Loop

			default:
				// gnore other events..
				//log.Printf("Unexpected: %v\n", msg.Data)
			}
		}
	}
}

func (s *SlackBot) RegisterEndpoint(endpoint string, method string, hf EPHandlerFunc) error {
	name := fmt.Sprintf("%v-%v", endpoint, method)
	if r := s.netcon.router.Get(name); r != nil {
		return errors.New("route exists")
	}
	switch strings.ToUpper(method) {
	case "GET":
		break
	case "POST":
		break
	default:
		return errors.New("invalid method, only accepting GET and POST")
		break
	}
	r := s.netcon.router.NewRoute()
	return r.Name(name).Path(endpoint).Methods(strings.ToUpper(method)).HandlerFunc(s.Wrap(hf)).GetError()
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

	r := mux.NewRouter()
	c := NetConn{}
	c.router = r

	newBot.netcon = &c

	//This Endpoint cannot be over-written
	newBot.RegisterEndpoint("/status", "get", func(rw http.ResponseWriter, r *http.Request, br *Response) {
		rw.Write([]byte("Evil Bot!"))
		br.RTM.PostMessage("#testing", "Someone's Touching Me!", newBot.MParams())
	})

	log.Printf("NewBot: %#v\n", newBot)
	return &newBot, nil
}
