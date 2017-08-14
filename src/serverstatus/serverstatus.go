package serverstatus

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"../bot"
	"../config"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/kidoman/go-steam"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

// Start begins the program logic
func Start() {
	//set each server status as online to start
	for i := range config.Config.Servers {
		config.Config.Servers[i].Online = true
		config.Config.Servers[i].HostDownCount = 0
	}

	err := bot.Session.UpdateStatus(0, config.Config.GameStatus)

	if err != nil {
		log.Println(err)
	}

	//start a new go routine
	go scanServers()
}

func scanServers() {

	//check if server are in config file
	if len(config.Config.Servers) < 1 {
		fmt.Println("No servers in config file.")
		return
	}

	// main infinite loop
	for {
		//load each server from config
		for index, host := range config.Config.Servers {
			//save the previous failure count
			prevHostDownCount := host.HostDownCount
			//if the host count reaches a certain limit then the host will be considered dead
			// and wait until a certain amount of retries have past to try again.
			if prevHostDownCount > config.Config.DeadHostCount {
				//TODO: change to a time limit wait
				if prevHostDownCount > config.Config.ResetRetryCount {
					//resets the dead host
					host.HostDownCount = 0
				} else {
					//dead host:
					config.Config.Servers[index].HostDownCount++
					config.Config.Servers[index].Online = false
					time.Sleep(time.Second * 2)
					continue
				}
			}

			//defaults the server to online
			serverUp := true

			//concats the port after host in address for net dialer
			host.Address = host.Address + ":" + strconv.Itoa(host.Port)
			server, err := steam.Connect(host.Address)

			fmt.Printf("Host: %v\n", host.Address)
			if err != nil {
				fmt.Printf("err bad host in config: %v\n", err)
				serverUp = false
			} else {
				serverUp = true
			}

			defer server.Close()

			//lame check to see if we should continue
			if serverUp {
				//first check ping
				ping, err := server.Ping()
				if err != nil {
					fmt.Printf("steam: could not ping %v: %v\n", host.Address, err)
					serverUp = false
					host.HostDownCount++
				} else {
					fmt.Printf("steam: ping to %v: %v\n", host.Address, ping)
					//second, check for server info (host game label/server name from app)
					info, err := server.Info()
					if err != nil {
						fmt.Printf("steam: could not get server info from %v: %v\n", host.Address, err)
					} else {
						fmt.Printf("steam: info of %v: %v\n", host.Address, info)
						//third, grab the players
						playersInfo, err := server.PlayersInfo()
						if err != nil {
							fmt.Printf("steam: could not get players info from %v: %v\n", host.Address, err)
						} else if len(playersInfo.Players) > 0 {
							fmt.Printf("steam: player infos for %v:\n", host.Address)
							for _, player := range playersInfo.Players {
								fmt.Printf("steam: %v %v\n", player.Name, player.Score)
							}
						}
						//if we can get to this step, we will declare the server up
						// if you are having troulbes with servers not going up, raise this up a few blocks
						host.HostDownCount = 0
					}
				}
			}

			//seems to be required to disconnect udp ports
			server.Close()

			if host.HostDownCount == config.Config.HostRetryCount {
				sendMessage(config.Config.RoleToNotify + " " + host.Name + " went offline!")
			} else if prevHostDownCount >= config.Config.HostRetryCount && host.HostDownCount == 0 {
				sendMessage(config.Config.RoleToNotify + " " + host.Name + " is now online!")
			}

			//remember the outcome of the server status check
			config.Config.Servers[index].HostDownCount = host.HostDownCount
			config.Config.Servers[index].Online = serverUp
			time.Sleep(time.Second * 2)
		}

		time.Sleep(time.Second * 5)
	}
}

func sendMessage(message string) {
	for _, roomID := range config.Config.RoomIDList {
		bot.Session.ChannelMessageSend(roomID, message)
	}
}

// MessageHandler function will be called every time a new
// message is created on any channel that the autenticated bot has access to.
func MessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	if m.Author.ID == bot.BotID {
		return
	}
	for _, roomID := range config.Config.RoomIDList {
		// only respond to channels in the config
		if m.ChannelID == roomID {
			if m.Content == "!ServerStatus" {
				for _, server := range config.Config.Servers {
					if server.Online {
						s.ChannelMessageSend(m.ChannelID, server.Name+" is online!")
					} else {
						s.ChannelMessageSend(m.ChannelID, server.Name+" is down!")
					}
				}
			}
		}
	}
}
