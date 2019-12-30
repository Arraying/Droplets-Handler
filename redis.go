package main

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/gomodule/redigo/redis"
)

type (
	// Payload represents a PUB/SUB event payload.
	Payload struct {
		Action string          `json:"a"` // The action. The data depends on this.
		Sender string          `json:"s"` // The sender.
		Data   json.RawMessage `json:"d"` // The raw payload data. This gets processed later.
		Token  string          `json:"t"`
	}
	// PayloadCreate is the data sent when the action is create.
	PayloadCreate struct {
		Template string `json:"x"` // The template name.
		Data     string `json:"v"` // The per-Droplet specific data.
	}
	// PayloadDelete is the data sent when the action is delete.
	PayloadDelete struct {
		Identifier string `json:"i"` // The identifier of the Droplet to delete.
	}
	// PayloadQuery is the data sent when the action is query.
	PayloadQuery struct {
		Droplets []*PayloadDroplet `json:"l"` // A list of currently online Droplets.
	}
	// PayloadDroplet represents a Droplet as a payload instance.
	PayloadDroplet struct {
		Identifier string `json:"i"` // The identifier of the Droplet.
		IP         string `json:"h"` // The IP address used to connect to the container.
		Port       int    `json:"p"` // The port of the container.
		Data       string `json:"v"` // The per-Droplet specific data.
	}
	// connections is a wrapper for multiple Redis connections.
	connections struct {
		normal redis.Conn        // A regular connection to dispatch commands in.
		event  *redis.PubSubConn // A connection that SUBs.
	}
)

const (
	payloadChannel        = "ch_dr" // The PUB/SUB channel that the handler, the Bungee and all Droplets use.
	payloadActionCreate   = "c"     // Create a Droplet.
	payloadActionDelete   = "d"     // Delete a Droplet.
	payloadActionIdentify = "i"     // Identify a Droplet.
	payloadActionQuery    = "q"     // Query all existing Droplets.
	payloadSenderHandler  = "#"     // This will be the unique sender ID of the handler.
)

// redisConnect connects to the Redis server, twice.
func redisConnect() {
	log.Println("Establishing normal connection")
	normalConnection, err := redisPrepare()
	if err != nil {
		panic(err)
	}
	log.Println("Establishing PUB/SUB connection.")
	pubSubConnection, err := redisPrepare()
	if err != nil {
		panic(err)
	}
	log.Println("Starting listener...")
	event := &redis.PubSubConn{Conn: pubSubConnection} // Upgrade to PUB/SUB conenction.
	err = event.Subscribe(payloadChannel)
	if err != nil {
		panic(err)
	}
	conns = connections{
		normal: normalConnection,
		event:  event,
	}
}

// redisPayloadListen is the listener of the PUB/SUB channel.
func redisPayloadListen() {
	for {
		switch data := conns.event.Receive().(type) {
		case redis.Message: // There's been a message.
			var payload Payload
			err := json.Unmarshal(data.Data, &payload)
			if err != nil {
				log.Printf("Error unmarshalling payload (not JSON?): %s.\n", err.Error())
			} else {
				redisPayloadHandle(&payload)
			}
		case error: // There's been an error. Most likely the Redis server shut down.
			if _, typeof := data.(*net.OpError); !typeof {
				log.Printf("Error listening to pub/sub: %s.\n", data.Error())
			}
			return

		}
	}
}

// redisPayloadHandle handles the payload.
func redisPayloadHandle(payload *Payload) {
	if payload.Sender == payloadSenderHandler {
		log.Println("Ignoring, source is own handler.")
		return
	}
	if payload.Token != config.Token {
		log.Println("Ignoring, wrong payload token.")
		return
	}
	switch payload.Action {
	case payloadActionCreate:
		var data PayloadCreate
		err := json.Unmarshal(payload.Data, &data)
		if err != nil {
			log.Printf("Could not unmarshal Droplet create data: %s.\n", err.Error())
			return
		}
	loop:
		for _, template := range templates {
			if template.Name == data.Template {
				go func() {
					droplet, err := template.construct(data.Data)
					if err != nil {
						log.Printf("Error creating Droplet %s: %s.\n", droplet.identifier, err.Error())
					}
					log.Printf("Successfully created Droplet %s.\n", droplet.identifier)
					err = droplet.run()
					log.Printf("Attempting to boot Dropelt %s.\n", droplet.identifier)
					if err != nil {
						log.Printf("Error booting Droplet %s: %s.\n", droplet.identifier, err.Error())
					}
					sleep := 120
					if config.Timings.Identify > 0 {
						sleep = config.Timings.Identify
					}
					go func() {
						identifier := droplet.identifier
						iid := droplet.iid
						time.Sleep(time.Duration(sleep) * time.Minute)
						current := store.get(identifier)
						if current != nil && !current.identified && current.iid == iid {
							log.Printf("Received no identify from Droplet %s, starting delete..", identifier)
							current.destroy(true)
						}
					}()
				}()
				break loop
			}
		}
	case payloadActionDelete:
		var data PayloadDelete
		err := json.Unmarshal(payload.Data, &data)
		if err != nil {
			log.Printf("Could not unmarshal Droplet delete data: %s.\n", err.Error())
			return
		}
		droplet := store.get(data.Identifier)
		if droplet == nil {
			log.Printf("Received request to delete invalid Droplet: %s.\n", payload.Data)
		} else {
			go droplet.destroy(false)
		}
	case payloadActionIdentify:
		var data PayloadDroplet
		err := json.Unmarshal(payload.Data, &data)
		if err != nil {
			log.Printf("Could not unmarshal Droplet identify data: %s.\n", err.Error())
			return
		}
		droplet := store.get(payload.Sender)
		if droplet == nil {
			log.Printf("Received request to identify invalid Droplet: %s.\n", data.Identifier)
		} else {
			droplet.identified = true
			log.Printf("Droplet %s identified, port: %v.\n", droplet.identifier, droplet.port)
		}
	case payloadActionQuery:
		data := &PayloadQuery{
			Droplets: make([]*PayloadDroplet, 0),
		}
		store.forEach(func(droplet *droplet) {
			if !droplet.identified {
				return
			}
			data.Droplets = append(data.Droplets, droplet.toPayloadData())
		})
		bytes, err := json.Marshal(data)
		if err != nil {
			log.Printf("Could not marshal droplet query data: %s.\n", err.Error())
			return
		}
		redisPayloadSend(&Payload{
			Action: payloadActionQuery,
			Sender: payloadSenderHandler,
			Data:   bytes,
			Token:  config.Token,
		})
	}
}

// redisPayloadSend sends payloads.
func redisPayloadSend(payload *Payload) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling payload: %s.\n", err.Error())
		return
	}
	str := string(bytes)
	_, err = conns.normal.Do("PUBLISH", payloadChannel, str)
	if err != nil {
		log.Printf("Error publishing payload: %s.\n", err.Error())
	}
}

// redisDisconnect disconnects from the Redis server.
func redisDisconnect() {
	conns.normal.Close()
	conns.event.Close()
}

// redisPrepare prepares a Redis connection with the specified credentials.
func redisPrepare() (connection redis.Conn, err error) {
	connection, err = redis.Dial("tcp", config.Redis.Host)
	if err != nil {
		return
	}
	log.Println("Established a new Redis connection.")
	_, err = connection.Do("AUTH", config.Redis.Auth)
	return
}
