package activitylog

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/wynwoodtech/evilbot/pkg/bot"
	"github.com/wynwoodtech/evilbot/pkg/storage"
)

type ActivityLogger struct {
	store *storage.Store
	s     *evilbot.SlackBot
}

func NewLogger(s *evilbot.SlackBot) error {
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

	if chs, err := s.RTM.GetChannels(false); err == nil {
		for _, ch := range chs {
			s.RTM.JoinChannel(ch.ID)
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
	if err := s.AddCmdHandler("seen", a.SeenHandler); err != nil {
		return err
	}

	s.AddCmdHandler("seen", func(e evilbot.Event, br *evilbot.Response) {
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

	s.RegisterEndpoint("/top5/{channelid}", "get", func(rw http.ResponseWriter, r *http.Request, br *evilbot.Response) {
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
				if cInfo, err := s.RTM.GetChannelInfo(ch); err == nil {
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
				if uInfo, err := s.RTM.GetUserInfo(strings.ToUpper(pu.Key)); err == nil {
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

	s.RegisterEndpoint("/bottom5/{channelid}", "get", func(rw http.ResponseWriter, r *http.Request, br *evilbot.Response) {
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
				if cInfo, err := s.RTM.GetChannelInfo(ch); err == nil {
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
				if uInfo, err := s.RTM.GetUserInfo(strings.ToUpper(pu.Key)); err == nil {
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
		if a.s.Logging {
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

func (a *ActivityLogger) TopFiveHandler(ev evilbot.Event, br *evilbot.Response) {
	if ev.Channel != nil {
		if strings.ToLower(ev.Channel.Name) == "general" ||
			strings.ToLower(ev.Channel.Name) == "random" {
			//br.ReplyToUser(&ev, "not available in this channel")
			//return
		}

		if p, err := a.TopFive(ev.Channel.ID); err == nil {
			var rtext string
			rtext += "Top 5 Active Users:\n"
			//br.SendToChannel(ev.Channel.ID, "Top 5 Active Users:")
			for _, pu := range p {
				if uInfo, err := a.s.RTM.GetUserInfo(strings.ToUpper(pu.Key)); err == nil {
					text := fmt.Sprintf("%v:\t\t\t%v", uInfo.Name, pu.Value)
					rtext += fmt.Sprintf("\t%v\n", text)
					//br.SendToChannel(ev.Channel.ID, text)
				}
			}
			br.SendToChannel(ev.Channel.ID, rtext)
			return
		}
	} else {
		br.ReplyToUser(&ev, "Only in a channel")
		return
	}
	br.ReplyToUser(&ev, "something went wrong")
}

func (a *ActivityLogger) BottomFiveHandler(ev evilbot.Event, br *evilbot.Response) {
	if ev.Channel != nil {
		if strings.ToLower(ev.Channel.Name) == "general" ||
			strings.ToLower(ev.Channel.Name) == "random" {
			//br.ReplyToUser(&ev, "not available in this channel")
			//return
		}
		if p, err := a.BottomFive(ev.Channel.ID); err == nil {
			var rtext string
			rtext += "Bottom 5 Active Users:\n"
			//br.SendToChannel(ev.Channel.ID, "Bottom 5 Active Users:")
			for _, pu := range p {
				if uInfo, err := a.s.RTM.GetUserInfo(strings.ToUpper(pu.Key)); err == nil {
					text := fmt.Sprintf("%v:\t\t\t%v", uInfo.Name, pu.Value)
					rtext += fmt.Sprintf("\t%v\n", text)
					//br.SendToChannel(ev.Channel.ID, text)
				}
			}
			br.SendToChannel(ev.Channel.ID, rtext)
			return
		}
	} else {

		br.ReplyToUser(&ev, "Only in a channel")
		return
	}
	br.ReplyToUser(&ev, "something went wrong")
}

func (a *ActivityLogger) SeenHandler(e evilbot.Event, r *evilbot.Response) {
	userString := strings.Split(e.ArgStr, " ")
	var userName string
	var userID string
	reg := regexp.MustCompile("<@([A-Z]\\w+)>")
	regUser := reg.FindSubmatch([]byte(userString[0]))
	if len(regUser) > 1 {
		userID = string(regUser[1])
	} else {
		userName = userString[0]
	}
	if len(userID) > 0 {
		user, err := r.RTM.GetUserInfo(userID)
		if err == nil {
			if t, err := a.Seen(user.ID); err == nil {
				lastSeen := t.Format("Mon Jan _2 2006 15:04")
				r.SendToChannel(e.Channel.Name, user.Name+" was last seen on: "+lastSeen)
				return
			}
		}
		r.ReplyToUser(&e, userID+" is not a valid user")
		return
	}
	if allUsers, err := r.RTM.GetUsers(); err == nil {
		for _, user := range allUsers {
			if user.Name != userName {
				continue
			}
			if t, err := a.Seen(user.ID); err == nil {
				lastSeen := t.Format("Mon Jan _2 2006 3:04 PM")
				r.SendToChannel(e.Channel.Name, user.Name+" was last seen on: "+lastSeen)
				return
			}
		}
	}
	r.ReplyToUser(&e, userName+" is not a valid user")
}

func (a *ActivityLogger) ActivityLogHandler(ev evilbot.Event, br *evilbot.Response) {
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
