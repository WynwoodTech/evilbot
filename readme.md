#EvilBot - Just Another SlackBot
----------

An easy way to launch extendable slackbots written in Go.

This is in early development.

v 0.1.1

Standard Feature List:
- [x] Command Handlers
- [x] General Handlers
- [x] Persistant Storage
- [x] HTTP Server with Chat & Http response to Endpoints
- [ ] HTTPS support and tokenized authentication 
- [ ] Built in user activity logger, catalogued by channel
- [ ] Top 5 Active/Inactive in channel/overall for activity logger 

Wynwood Tech E-Bot Sepcific:
- [ ] Modify our ranking algorithm to have some smarts. 
- [ ] Interact with the starndard slackbot in some way. 

Getting Started:

1. go get github.com/wynwoodtech/evilbot
2. cd $GOPATH/src/github.com/wynwoodtech/evilbot
3. export SLACKBOT=your_slack_api_key
4. go run cmd/main.go


License: GPLv3

Using Libraries:
[nlopes/slack](http://github.com/nlopes/slack)

[boltdb/bolt](http://github.com/boltdb/bolt)

[gorilla/mux](http://github.com/gorilla/mux)



Created By [Liran Cohen](http://www.github.com/lirancohen) (c)2016 For [#WynwoodTech](http://www.wyn.tech)

