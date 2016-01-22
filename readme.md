#EvilBot - Just Another SlackBot
----------

An easy way to launch extendable slackbots written in Go.

This is in early development.

v 0.1.1

Standard Feature List:
- [x] Command Handlers
- [x] General Handlers
- [ ] Persistant Storage
- [ ] HTTPS server with tokenized auth for stats/triggers 
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
