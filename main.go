package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/plouc/go-jira-client"
	irc "github.com/thoj/go-ircevent"
	goyaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
	"time"
)

// Config struct to store configurable data in a yaml file
type Config struct {
	Host         string   `yaml:"host"`
	ApiPath      string   `yaml:"api_path"`
	ActivityPath string   `yaml:"activity_path"`
	Login        string   `yaml:"login"`
	Password     string   `yaml:"password"`
	IRCHostname  string   `yaml:"irc_hostname"`
	IRCPass      string   `yaml:"irc_pass"`
	IRCUsername  string   `yaml:"irc_username"`
	IRCNick      string   `yaml:"irc_nick"`
	IRCNickPass  string   `yaml:"irc_nick_pass"`
	IRCChannels  []string `yaml:"irc_channels"`
}

var (
	imgur_ext_regex = regexp.MustCompile(`.*\.\w\w\wv?$`)
	url_regex       = regexp.MustCompile(`\b(https?://?[\dA-Za-z\.-]+[/\w\.\+\?\:\;\#\&=-]*)`)
	jira_regex      = regexp.MustCompile(`^https?://jira2.advance.net/browse/([a-zA-Z0-9-]+)?.*$`)
	config          = new(Config)
)

func jiraScrape(url string, messageCh chan string) {
	jira := gojira.NewJira(
		config.Host,
		config.ApiPath,
		config.ActivityPath,
		&gojira.Auth{config.Login, config.Password},
	)
	if jira_regex.MatchString(url) {
		match := jira_regex.FindStringSubmatch(url)[1]
		log.Printf("Match: '%s'\n", match)
		issue := jira.Issue(match)
		//log.Printf("Jira: %#v\n", issue.Fields)
		messageCh <- fmt.Sprintf("%s - %s\n", issue.Key, issue.Fields.Summary)
		// TODO: Check len on the following to avoid flooding
		//fmt.Printf("%s\n", issue.Fields.Summary)
	} else {
		log.Printf("Can't find JIRA match for: %s\n", url)
	}
}

// youtubeScrape get's the title from a youtube video page.
func youtubeScrape(url string, messageCh chan string) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		log.Print(err)
	}
	doc.Find("#eow-title").Each(func(i int, s *goquery.Selection) {
		log.Printf("Youtube: %s\n", strings.Trim(s.Text(), "\n "))
		messageCh <- fmt.Sprintf("YouTube: %s\n", strings.Trim(s.Text(), "\n "))
	})
}

// imgurScrape get's the title from a imgur album page.
func imgurScrape(url string, messageCh chan string) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		log.Print(err)
	}
	doc.Find("div#content .album-description").Each(func(i int, s *goquery.Selection) {
		log.Printf("Imgur: %s\n", strings.Trim(s.Find("h1").Text(), "\n "))
		messageCh <- fmt.Sprintf("Imgur: %s\n", strings.Trim(s.Find("h1").Text(), "\n "))
	})
}

func vimgurScrape(url string, messageCh chan string) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		log.Print(err)
	}
	doc.Find("head title").Each(func(i int, s *goquery.Selection) {
		log.Printf("Imgur: %s\n", strings.Trim(s.Text(), "\n "))
		messageCh <- fmt.Sprintf("Imgur: %s\n", strings.Trim(s.Text(), "\n "))
	})
}

// scrapePage figures out what kind of page we're dealing with and calls the appropriate func.
func scrapePage(url string, messageCh chan string) {
	switch {
    // Commented out youtube since we have another bot that does this in channel
	//	case strings.Contains(url, "youtu.be"):
	//		youtubeScrape(url, messageCh)
	//	case strings.Contains(url, "youtube"):
	//		youtubeScrape(url, messageCh)

	case strings.Contains(url, "imgur.com/a"):
		imgurScrape(url, messageCh)
	case strings.Contains(url, "imgur.com/gallery"):
		vimgurScrape(url, messageCh)
	case strings.Contains(url, ".gifv"): // .gifv images are actually pages
		vimgurScrape(url, messageCh)
	case imgur_ext_regex.MatchString(url): // has an extension (ex. .jpg) but not a .gifv
		vimgurScrape(url[:len(url)-3]+"gifv", messageCh) // Remove the extension and add a .gifv which seems to work

	case strings.Contains(url, "jira2.advance.net"):
		jiraScrape(url, messageCh)
	default:
		log.Printf("Unknown url type: %s\n", url)
	}
}

// UnmarshalConfig reads in the config.yaml file
func UnmarshalConfig(filename string) {
	file, e := ioutil.ReadFile(filename)
	if e != nil {
		log.Fatalf("Problem reading config file: %v\n", e)
	}

	err := goyaml.Unmarshal(file, &config)
	if err != nil {
		log.Fatalf("Problem unmarshalling config file: %v\n", err)
	}
}

func ConnectIRC() *irc.Connection {
	con := irc.IRC(config.IRCNick, config.IRCUsername)
	if config.IRCPass != "" {
		con.Password = config.IRCPass
	}
	err := con.Connect(config.IRCHostname)
	if err != nil {
		log.Fatalf("Problem connecting to irc: %s", err)
	}
	return con
}

// 001 is the welcome even, join channels when we're logged in
func WelcomeCallback(con *irc.Connection) {
	con.AddCallback("001", func(e *irc.Event) {
		con.Privmsgf("NickServ", "identify %s", config.IRCNickPass)
		time.Sleep(time.Second * 15) // Need to wait a little for the registration to take effect

		for _, room := range config.IRCChannels {
			con.Join(room)
			log.Printf("Connected to channel %s\n", room)
		}
	})
}

// JOIN event happens whenever *anyone* joins
func JoinCallback(con *irc.Connection) {
	con.AddCallback("JOIN", func(e *irc.Event) {
		if e.Nick != config.IRCNick { // Exclude bot joins
			log.Printf("%s joined %s", e.Nick, e.Arguments[0])
		} else {
			log.Printf(e.Message())
		}
	})
}

// PRIVMSG is really any message in IRC that the bot sees
func PrivMsgCallback(con *irc.Connection) {
	con.AddCallback("PRIVMSG", func(e *irc.Event) {
		message := e.Arguments[1]
		channel := e.Arguments[0]
		log.Printf("%s said: %s\n", e.Nick, message)

		// Look for urls in messages
		urls := url_regex.FindAllString(message, -1)
		if len(urls) > 0 {
			for _, url := range urls {
				messageCh := make(chan string, 1)
				go scrapePage(url, messageCh)
				con.Privmsg(channel, <-messageCh)
			}
		}
	})
}

// Not sure if this actually works
func ErrorCallback(con *irc.Connection) {
	con.AddCallback("ERROR", func(e *irc.Event) {
		log.Printf("Error: %#v\n", e)
	})
}

func main() {
	UnmarshalConfig("config.yaml")
	con := ConnectIRC()

	WelcomeCallback(con)
	JoinCallback(con)
	PrivMsgCallback(con)
	ErrorCallback(con)

	con.Loop()
}
