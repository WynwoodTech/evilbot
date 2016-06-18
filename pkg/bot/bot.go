package evilbot

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/nlopes/slack"
)

func (r *Response) MParams() slack.PostMessageParameters {
	msgParams := slack.NewPostMessageParameters()
	msgParams.AsUser = true
	msgParams.LinkNames = 1
	return msgParams
}

type SlackBot struct {
	name     string
	Logging  bool
	leadchar string
	userid   string
	conn     *slack.Client
	RTM      *slack.RTM
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
		rb.RTM = s.RTM
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
	Bot *SlackBot
}

//Reply's to User.
//If in prvt message it will respond there
//If in channel will mention user.
func (r *Response) ReplyToUser(e *Event, text string) error {
	p := r.MParams()
	var rID string
	if e.Channel == nil {
		rID = e.User.ID
	} else {
		rID = e.Channel.ID
		text = fmt.Sprintf("@%v: %v", e.User.Name, text)
	}
	if _, _, err := r.RTM.PostMessage(rID, text, p); err != nil {
		return err
	}
	return nil

}
func (r *Response) UserInfo(uname string) (slack.User, error) {
	if users, err := r.RTM.GetUsers(); err == nil {
		for _, u := range users {
			if strings.ToLower(u.Name) == strings.ToLower(uname) {
				return u, nil
			}
		}
	}
	return slack.User{}, errors.New("could not find member")
}

//Get Channel Info by channel name
func (r *Response) ChannelInfo(cname string) (slack.Channel, error) {
	if chs, err := r.RTM.GetChannels(false); err == nil {
		for _, ch := range chs {
			if strings.ToLower(ch.Name) == strings.ToLower(cname) {
				return ch, nil
			}
		}
	}
	return slack.Channel{}, errors.New("could not find channel")
}

//Send A Message to a channel given a channel name or channel ID and the text
func (r *Response) SendToChannel(channel string, text string) error {
	p := r.MParams()
	if _, _, err := r.RTM.PostMessage(channel, text, p); err != nil {
		return err
	}
	return nil
}

func (s *SlackBot) CurrentChannels() []slack.Channel {
	response := []slack.Channel{}
	if chs, err := s.RTM.GetChannels(true); err == nil {
		for _, ch := range chs {
			for _, u := range ch.Members {
				log.Printf("User: %v\n", u)
				log.Printf("Me  : %v\n", s.userid)
				if u == s.userid {
					response = append(response, ch)
				}
			}
		}
	}
	return response
}

func (s *SlackBot) JoinEvent(e *slack.TeamJoinEvent) {
	ev := Event{}
	ev.User = e.User
	s.HandleJoin(ev)
}

func (s *SlackBot) HandleJoin(e Event) {
	time.Sleep(600 * time.Millisecond)
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
		e.User.Name,
		phrases[rand.Intn(len(phrases))],
	)
	s.RTM.PostMessage("#general", msgString, msgParams)
	msgString = "I am Evil Bot here to serve and entertain. My commands are largely Easter-eggs... but you can always ask for !top5."
	s.RTM.PostMessage("#general", msgString, msgParams)
}
func (s *SlackBot) HandleEvent(e *slack.MessageEvent) {
	ev := Event{}
	ev.LeadChar = s.leadchar
	ev.User, _ = s.RTM.GetUserInfo(e.User)
	ev.Channel, _ = s.RTM.GetChannelInfo(e.Channel)
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
			r.RTM = s.RTM
			cmd.Handle.ServeHandler(e, &r)
		}
	}
}

func (s *SlackBot) HandleMsg(e Event) {
	for _, ev := range s.handlers {
		r := Response{}
		r.RTM = s.RTM
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

func (s *SlackBot) SetLogging(b bool) {
	s.Logging = b
}
func (s *SlackBot) Run() {
	s.run()
}
func (s *SlackBot) RunWithHTTP(port string) {

	go http.ListenAndServe(fmt.Sprintf(":%v", port), s.netcon.router)
	s.run()

}

func (s *SlackBot) run() {

	go s.RTM.ManageConnection()
Loop:
	for {
		select {
		case msg := <-s.RTM.IncomingEvents:
			if s.Logging {
				log.Printf("Logger: %#v\n", msg.Data)
			}
			switch ev := msg.Data.(type) {
			case *slack.TeamJoinEvent:
				go s.JoinEvent(ev)

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
	newBot := SlackBot{}
	api := slack.New(apiKey)
	rtm := api.NewRTM()
	if at, err := rtm.AuthTest(); err != nil {
		return nil, err
	} else {
		newBot.userid = at.UserID
	}

	newBot.name = apiKey
	newBot.conn = api
	newBot.RTM = rtm
	newBot.leadchar = leadChars

	r := mux.NewRouter()
	c := NetConn{}
	c.router = r

	newBot.netcon = &c

	//This Endpoint cannot be over-written
	newBot.RegisterEndpoint("/status", "get", func(rw http.ResponseWriter, r *http.Request, br *Response) {
		rw.Write([]byte("Evil Bot!"))
		br.RTM.PostMessage("#testing", "Someone's Touching Me!", br.MParams())
	})

	newBot.AddCmdHandler("channels", func(e Event, br *Response) {
		var rtext string
		rtext += "Channels I'm In:\n"
		//br.SendToChannel(e.Channel.ID, "Channels I'm In:")
		for _, ch := range newBot.CurrentChannels() {
			//br.SendToChannel(e.Channel.ID, ch.Name)
			rtext += fmt.Sprintf("\t%v\n", ch.Name)
		}
		br.SendToChannel(e.Channel.ID, rtext)
	})

	log.Printf("NewBot: %#v\n", newBot)
	return &newBot, nil
}
