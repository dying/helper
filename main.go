package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dying/helper/config"
	"golang.org/x/time/rate"
)

// Create a custom channel struct which holds the rate limiter for each
// channel and the last time that the channel was seen.
type channel struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Change the the map to hold values of the type channnel.
var channels = make(map[string]*channel)
var mtx sync.Mutex

/* Tries to call a method and checking if the method returned an error, if it
did check to see if it's HTTP 502 from the Discord API and retry for
`attempts` number of times. */
func retryOnBadGateway(f func() error) {
	var err error
	for i := 0; i < 3; i++ {
		err = f()
		if err != nil {
			if strings.HasPrefix(err.Error(), "HTTP 502") {
				// If the error is Bad Gateway, try again after 1 sec.
				time.Sleep(1 * time.Second)
				continue
			} else {
				// Otherwise panic !
				if err != nil {
					panic(err)
				}
			}
		} else {
			// In case of no error, return.
			return
		}
	}
}

// fetchGuild fetch the given guild
// sess : The DiscordGo Session
// guildID : The ID of a Guild.
func fetchGuild(sess *discordgo.Session, guildID string) *discordgo.Guild {
	var result *discordgo.Guild
	retryOnBadGateway(func() error {
		var err error
		result, err = sess.Guild(guildID)
		if err != nil {
			return err
		}
		return nil
	})
	return result
}

// fetchUser fetch the given User
// sess : The DiscordGo Session
// userID : The ID of a User.
func fetchUser(sess *discordgo.Session, userID string) *discordgo.User {
	var result *discordgo.User
	retryOnBadGateway(func() error {
		var err error
		result, err = sess.User(userID)
		if err != nil {
			return err
		}
		return nil
	})
	return result
}

// fetchChannel fetch the given Channel
// sess : The DiscordGo Session
// userID : The ID of a User.
func fetchChannel(sess *discordgo.Session, channelID string) *discordgo.Channel {
	var result *discordgo.Channel
	retryOnBadGateway(func() error {
		var err error
		result, err = sess.Channel(channelID)
		if err != nil {
			return err
		}
		return nil
	})
	return result
}

func main() {

	provider := config.GetConfigProvider()
	provider.LoadConfig()
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + config.Conf.BotToken)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Run a background goroutine to remove old entries from the channels map.
	go cleanupChannels(dg)

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	// Register the memberJoin func as a callback for GuildMemberAdd events.
	dg.AddHandler(memberJoin)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// fmt.Println("MESSAGE CREATE FIRED!")

	// Fetch the guild directly to the Gateway
	guild := fetchGuild(s, m.GuildID)

	// Fetch the channel to ratelimit
	limiter := getChannel(m.ChannelID)

	// Find @everyone role and deny them access
	var everyone string
	for _, v := range guild.Roles {
		if v.Name == "@everyone" {
			everyone = v.ID
			break
		}
	}

	// If the ratelimit is hit, lockdown the channel
	if limiter.Allow() == false {
		s.ChannelPermissionSet(m.ChannelID, everyone, "role", 0, 2048)
	}

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}
	// If the message is "ping" reply with "Pong!"
	if m.Content == "h!ping" {
		s.ChannelMessageSend(m.ChannelID, "Pong!")
	}

	// If the message start with "h!find", runs the command
	if strings.HasPrefix(m.Content, "h!find") {
		// Take everything after the command
		args := strings.TrimPrefix(m.Content, "h!find ")

		// Create a slice
		member := []string{}

		// Fetch the guild
		g, err := s.State.Guild(m.GuildID)
		if err != nil {
			g, err = s.Guild(m.GuildID)
			if err == nil {
				g.Members = nil
			}
		}

		if err != nil {
			fmt.Println("Error while fetching the guild:", err)
		}

		if len(g.Members) == 0 {
			recurseMembers(s, &g.Members, m.GuildID, "0")
		}

		// Loop to check every user in the guild that starting with the args
		for _, u := range g.Members {
			if strings.HasPrefix(u.User.Username, args) {
				// Add the userID to the slice
				member = append(member, u.User.ID)
				fmt.Println("Found a user starting with:", args, "and his username is:", u.User.Username)
			}
		}

		if len(member) == 0 {
			s.ChannelMessageSend(m.ChannelID, "Couldn't find anyone with this username.")
			return
		}

		s.ChannelMessageSend(m.ChannelID, "There are IDs of people starting with: "+args+" and here is the array: "+strings.Join(member[:], ", "))
	}

	// If the message start with "h!ban", runs the command
	if strings.HasPrefix(m.Content, "h!ban") {
		// Take everything after the command
		args := strings.TrimPrefix(m.Content, "h!ban ")

		members := strings.Split(args, ", ")

		// Loop to ban every user in the slice
		for _, u := range members {
			err := s.GuildBanCreate(m.GuildID, u, 7)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "Failed to ban "+u)
				fmt.Println("Failed to ban "+u, err)
				return
			}
			s.ChannelMessageSend(m.ChannelID, "Banned"+u)
		}
	}

}

// This function will be called (due to AddHandler above) every time a new
// member is created on any guild that the autenticated bot has access to.
func memberJoin(s *discordgo.Session, evt *discordgo.GuildMemberAdd) {
	created, err := SnowflakeTimestamp(evt.Member.User.ID)
	if err != nil {
		fmt.Print("Couldn't decode snowflake ID", err)
	}
	// Kick automatically every user that starts with the keyword "Raid"
	if strings.HasPrefix(evt.Member.User.Username, "Raid") {
		err := s.GuildMemberDeleteWithReason(evt.GuildID, evt.Member.User.ID, "Username starts with 'Raid'.")
		if err != nil {
			fmt.Println("Kick failed", err)
			return
		}
	}

	// Kick the user if his account is less than 72 hours old
	if time.Now().Sub(created).Hours() < 72 {
		err := s.GuildMemberDeleteWithReason(evt.GuildID, evt.Member.User.ID, "Account created less than 72 hours ago.")
		if err != nil {
			fmt.Println("Kick failed", err)
		}
	}
}

// SnowflakeTimestamp returns the creation time of a Snowflake ID relative to the creation of Discord.
// https://github.com/Necroforger/discordgo/blob/e3acfe56f06abc4eff33d61b2fd8c1696a1b0126/util.go
func SnowflakeTimestamp(ID string) (t time.Time, err error) {
	i, err := strconv.ParseInt(ID, 10, 64)
	if err != nil {
		return
	}
	// 1420070400000 = Discord Epoch
	timestamp := (i >> 22) + 1420070400000
	t = time.Unix(timestamp/1000, 0)
	return
}

// recurseMembers Bypass the cap of 1k from Discord to get the users.
func recurseMembers(s *discordgo.Session, memstore *[]*discordgo.Member, guildID, after string) {
	members, err := s.GuildMembers(guildID, after, 1000)
	if err != nil {
		fmt.Println(err)
		return
	}

	if len(members) == 1000 {
		recurseMembers(
			s,
			memstore,
			guildID,
			members[999].User.ID,
		)
	}

	*memstore = append(*memstore, members...)

	return
}
