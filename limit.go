package main

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/time/rate"
)

func addChannel(id string) *rate.Limiter {
	// Allows 20 tokens to be consumed/s, with a maxiumum burst size of 40
	limiter := rate.NewLimiter(20, 40)
	mtx.Lock()
	// Include the current time when creating a new channel.
	channels[id] = &channel{limiter, time.Now()}
	mtx.Unlock()
	return limiter
}

func getChannel(id string) *rate.Limiter {
	mtx.Lock()
	v, exists := channels[id]
	if !exists {
		mtx.Unlock()
		return addChannel(id)
	}

	// Update the last seen time for the channel.
	v.lastSeen = time.Now()
	mtx.Unlock()
	return v.limiter
}

// Every minute check the map for channels that haven't been seen for
// more than 3 minutes and delete the entries.
func cleanupChannels(s *discordgo.Session) {
	for {
		time.Sleep(time.Minute)
		mtx.Lock()
		for id, v := range channels {
			// channel := fetchChannel(s, id)
			// guild := fetchGuild(s, channel.GuildID)
			if time.Now().Sub(v.lastSeen) > 3*time.Minute {
				/*
					var everyone string
					for _, v := range guild.Roles {
						if v.Name == "@everyone" {
							everyone = v.ID
							break
						}
					}
					// Ratelimit is over, give the right to send message back
					// s.ChannelPermissionSet(id, everyone, "role", 2048, 0)
				*/
				delete(channels, id)
			}
		}
		mtx.Unlock()
	}
}
