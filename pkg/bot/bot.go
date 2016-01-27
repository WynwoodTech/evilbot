package evilbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wynwoodtech/evilbot/pkg/storage"

	"github.com/boltdb/bolt"
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
	logging  bool
	leadchar string
	userid   string
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
	if chs, err := s.rtm.GetChannels(true); err == nil {
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

func (s *SlackBot) ActivityLogger() error {
	a := ActivityLogger{}
	a.s = s
	var err error
	a.store, err = storage.Load("activity_logger")
	if err != nil {
		return err
	}
	if err := a.store.LoadBucket("all"); err != nil {
		return err
	}
	if err := a.store.LoadBucket("seen"); err != nil {
		return err
	}

	if chs, err := s.rtm.GetChannels(false); err == nil {
		for _, ch := range chs {
			s.rtm.JoinChannel(ch.ID)
			log.Printf("Loading Bucket: %v\n", ch.ID)
			if err := a.store.LoadBucket(ch.ID); err != nil {
				return err
			}
		}
	}
	if err := s.AddEventHandler(
		"default-activity-logger",
		a.ActivityLogHandler,
	); err != nil {
		return err
	}
	if err := s.AddCmdHandler("top5", a.TopFiveHandler); err != nil {
		return err
	}
	if err := s.AddCmdHandler("bottom5", a.BottomFiveHandler); err != nil {
		return err
	}

	s.AddCmdHandler("seen", func(e Event, br *Response) {
		var response string
		if e.Channel != nil {
			log.Printf("Args: %v", e.ArgStr)
			if len(e.ArgStr) > 0 {
				u := strings.Split(e.ArgStr, " ")
				if uInfo, err := br.UserInfo(u[0]); err == nil {
					if t, err := a.Seen(strings.ToLower(uInfo.ID)); err == nil {
						tf := t.Format("Jan 2 2006 03:04 PM EST")
						response = fmt.Sprintf("%v last seen on %v", u[0], tf)
						br.SendToChannel(e.Channel.ID, response)
						return
					}
				} else {
					log.Printf("Error: %v\n", err)
				}
				response = fmt.Sprintf("%v not seen", u[0])
			} else {
				response = "which user?"
			}
		} else {
			response = "only in channel"
		}
		br.ReplyToUser(&e, response)
	})

	s.RegisterEndpoint("/top5/{channelid}", "get", func(rw http.ResponseWriter, r *http.Request, br *Response) {
		ch := mux.Vars(r)["channelid"]
		if len(ch) < 0 {
			ch = "all"
		}
		if ch != "all" {
			chInfo, err := br.ChannelInfo(ch)
			if err == nil {
				ch = chInfo.ID
			} else {
				rw.Write([]byte("Evil Bot Says No!"))
				return
			}
		}

		if p, err := a.TopFive(ch); err == nil {
			var cname string
			if ch != "all" {
				if cInfo, err := s.rtm.GetChannelInfo(ch); err == nil {
					cname = cInfo.Name
				} else {
					rw.Write([]byte("Evil Bot Says No!"))
					return
				}
			} else {
				cname = "all activity"
			}
			response := map[string]interface{}{
				"channel": cname,
				"results": p,
			}
			for i, pu := range p {
				if uInfo, err := s.rtm.GetUserInfo(strings.ToUpper(pu.Key)); err == nil {
					p[i] = Pair{
						Key:   uInfo.Name,
						Value: pu.Value,
					}
				} else {
					log.Printf("Eorror: %v\n", err)
				}
			}
			w, err := json.Marshal(response)
			if err == nil {
				rw.Write(w)
				return
			}
		}
		rw.Write([]byte("Evil Bot Says No!"))
	})

	s.RegisterEndpoint("/bottom5/{channelid}", "get", func(rw http.ResponseWriter, r *http.Request, br *Response) {
		ch := mux.Vars(r)["channelid"]
		if len(ch) < 0 {
			ch = "all"
		}
		if ch != "all" {
			chInfo, err := br.ChannelInfo(ch)
			if err == nil {
				ch = chInfo.ID
			} else {
				rw.Write([]byte("Evil Bot Says No!"))
				return
			}
		}
		if p, err := a.BottomFive(ch); err == nil {
			var cname string
			if ch != "all" {
				if cInfo, err := s.rtm.GetChannelInfo(ch); err == nil {
					cname = cInfo.Name
				} else {
					rw.Write([]byte("Evil Bot Says No!"))
					return
				}
			} else {
				cname = "all activity"
			}
			response := map[string]interface{}{
				"channel": cname,
				"results": p,
			}
			for i, pu := range p {
				if uInfo, err := s.rtm.GetUserInfo(strings.ToUpper(pu.Key)); err == nil {
					p[i] = Pair{
						Key:   uInfo.Name,
						Value: pu.Value,
					}
				} else {
					log.Printf("Eorror: %v\n", err)
				}
			}
			w, err := json.Marshal(response)
			if err == nil {
				rw.Write(w)
				return
			}
		}
		rw.Write([]byte("Evil Bot!"))
	})
	return nil
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
	newBot.rtm = rtm
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
		br.SendToChannel(e.Channel.ID, "Channels I'm In:")
		for _, ch := range newBot.CurrentChannels() {
			br.SendToChannel(e.Channel.ID, ch.Name)
		}
	})

	log.Printf("NewBot: %#v\n", newBot)
	return &newBot, nil
}

type ActivityLogger struct {
	store *storage.Store
	s     *SlackBot
}

func (a *ActivityLogger) logtime(u string) error {
	t, err := time.Now().MarshalText()
	if err != nil {
		log.Println("Time Error")
		return err
	}
	if err := a.store.SetVal("seen", u, string(t)); err != nil && err.Error() != "no value" {
		log.Println("SetVal Error")
		return err
	}
	return nil
}

func (a *ActivityLogger) Seen(u string) (time.Time, error) {
	t := time.Time{}
	s, err := a.store.GetVal("seen", u)
	if err != nil {
		log.Println("GetVal Error")
		return t, err
	}
	if err := t.UnmarshalText([]byte(s)); err != nil {
		return t, err
	}
	return t, nil
}

func (a *ActivityLogger) log(c string, u string) error {
	s, err := a.store.GetVal(c, u)
	if err != nil && err.Error() != "no value" {
		log.Println("GetVal Error")
		return err
	}
	if i, err := strconv.Atoi(s); err == nil || len(s) == 0 {
		if len(s) == 0 {
			i = 1
		} else {
			i++
		}
		if a.s.logging {
			log.Printf("Logger: %v %v Count - %v\n", c, u, i)
		}
		if err := a.store.SetVal(c, u, strconv.Itoa(i)); err != nil {
			log.Println("SetVal Error")
			return err
		}
	}
	return nil
}

func (a *ActivityLogger) BottomFive(channel string) (PairList, error) {
	p := PairList{}
	channel = strings.ToLower(channel)
	if err := a.store.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(channel))

		if err := b.ForEach(func(k, v []byte) error {
			u := string(k)
			if u == "all" || u == "slackbot" {
				return nil
			}
			vi, err := strconv.Atoi(string(v))
			if err != nil {
				return err
			}
			if vi > 0 {
				p = append(p, Pair{u, vi})
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return p, err
	}
	sort.Sort(p)
	var c int
	if len(p) > 5 {
		c = 5
	} else {
		c = len(p)
	}
	return p[:c], nil
}

func (a *ActivityLogger) TopFive(channel string) (PairList, error) {
	channel = strings.ToLower(channel)
	p := PairList{}
	if err := a.store.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(channel))

		if err := b.ForEach(func(k, v []byte) error {
			u := string(k)
			if u == "all" || u == "slackbot" {
				return nil
			}
			vi, err := strconv.Atoi(string(v))
			if err != nil {
				return err
			}
			if vi > 0 {
				p = append(p, Pair{u, vi})
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return p, err
	}
	sort.Sort(sort.Reverse(p))
	var c int
	if len(p) > 5 {
		c = 5
	} else {
		c = len(p)
	}
	return p[:c], nil
}

func (a *ActivityLogger) TopFiveHandler(ev Event, br *Response) {
	if ev.Channel != nil {
		if p, err := a.TopFive(ev.Channel.ID); err == nil {
			br.SendToChannel(ev.Channel.ID, "Top 5 Active Users:")
			for _, pu := range p {
				if uInfo, err := a.s.rtm.GetUserInfo(strings.ToUpper(pu.Key)); err == nil {
					text := fmt.Sprintf("%v: %v", uInfo.Name, pu.Value)
					br.SendToChannel(ev.Channel.ID, text)
				}
			}
			return
		}
	} else {
		br.ReplyToUser(&ev, "Only in a channel")
		return
	}
	br.ReplyToUser(&ev, "something went wrong")
}

func (a *ActivityLogger) BottomFiveHandler(ev Event, br *Response) {
	if ev.Channel != nil {
		if p, err := a.BottomFive(ev.Channel.ID); err == nil {
			br.SendToChannel(ev.Channel.ID, "Bottom 5 Active Users:")
			for _, pu := range p {
				if uInfo, err := a.s.rtm.GetUserInfo(strings.ToUpper(pu.Key)); err == nil {
					text := fmt.Sprintf("%v: %v", uInfo.Name, pu.Value)
					br.SendToChannel(ev.Channel.ID, text)
				}
			}
			return
		}
	} else {

		br.ReplyToUser(&ev, "Only in a channel")
		return
	}
	br.ReplyToUser(&ev, "something went wrong")
}

func (a *ActivityLogger) ActivityLogHandler(ev Event, br *Response) {
	if ev.Channel != nil && ev.User != nil {
		if !ev.User.IsBot {
			go func() {
				if err := a.logtime(ev.User.ID); err != nil {
					log.Printf("LogTime Log Error: %v\n", err.Error())
					log.Printf("\tDetails [User: %v, UserID: %v, Channel: %v, ChannelID: %v]\n",
						ev.User.Name, ev.User.ID, ev.Channel.Name, ev.Channel.ID)
				}
			}()
			go func() {
				if err := a.log(ev.Channel.ID, ev.User.ID); err != nil {
					log.Printf("Channel/User Log Error: %v\n", err.Error())
					log.Printf("\tDetails [User: %v, UserID: %v, Channel: %v, ChannelID: %v]\n",
						ev.User.Name, ev.User.ID, ev.Channel.Name, ev.Channel.ID)
				}
			}()
			go func() {
				if err := a.log("all", ev.User.ID); err != nil {
					log.Printf("All/User Log Error: %v\n", err.Error())
					log.Printf("\tDetails [User: %v, UserID: %v, Channel: %v, ChannelID: %v]\n",
						ev.User.Name, ev.User.ID, ev.Channel.Name, ev.Channel.ID)
				}
			}()
			go func() {
				if err := a.log(ev.Channel.ID, "all"); err != nil {
					log.Printf("Channel/All Log Error: %v\n", err.Error())
					log.Printf("\tDetails [User: %v, UserID: %v, Channel: %v, ChannelID: %v]\n",
						ev.User.Name, ev.User.ID, ev.Channel.Name, ev.Channel.ID)
				}
			}()
			go func() {
				if err := a.log("all", "all"); err != nil {
					log.Printf("All/All Log Error: %v\n", err.Error())
					log.Printf("\tDetails [User: %v, UserID: %v, Channel: %v, ChannelID: %v]\n",
						ev.User.Name, ev.User.ID, ev.Channel.Name, ev.Channel.ID)
				}
			}()
		}
	}
}

//List Helper
type Pair struct {
	Key   string
	Value int
}

// A slice of Pairs that implements sort.Interface to sort by Value.
type PairList []Pair

func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
