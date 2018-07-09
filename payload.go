package main

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/gomodule/redigo/redis"
)

// payloadRecieve starts receiving and handling payloads.
func payloadReceive() {
	for {
		switch data := conns.pubSub.Receive().(type) {
		case redis.Message:
			var payload Payload
			err := json.Unmarshal(data.Data, &payload)
			if err != nil {
				log.Printf("Error unmarshalling payload (not JSON?): %s.\n", err.Error())
			} else {
				payloadHandle(&payload)
			}
		case error:
			if _, typeof := data.(*net.OpError); !typeof {
				log.Printf("Error listening to pub/sub: %s.\n", data.Error())
			}
			return
		}
	}
}

// payloadSend sends a payload.
func payloadSend(payload *Payload) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling payload: %s.\n", err.Error())
		return
	}
	str := string(bytes)
	_, err = conns.regular.Do("PUBLISH", payloadChannel, str)
	if err != nil {
		log.Printf("Error publishing payload: %s.\n", err.Error())
	}
}

// payloadHandle handles a payload.
func payloadHandle(payload *Payload) {
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
		var data PayloadCreateData
		err := json.Unmarshal(payload.Data, &data)
		if err != nil {
			log.Printf("Could not unmarshal droplet create data: %s.\n", err.Error())
			return
		}
	loop:
		for _, template := range templates {
			if template.Name == data.Template {
				go func() {
					droplet, err := template.create(data.Data)
					if err != nil {
						log.Printf("Error creating droplet %s: %s.\n", droplet.identifier, err.Error())
					} else {
						log.Printf("Successfully created droplet %s.\n", droplet.identifier)
						err = droplet.boot()
						log.Printf("Attempting to boot dropelt %s.\n", droplet.identifier)
						if err != nil {
							log.Printf("Error booting droplet %s: %s.\n", droplet.identifier, err.Error())
						}
						go func() {
							identifier := droplet.identifier
							iid := droplet.iid
							time.Sleep(2 * time.Minute)
							current := droplets.get(identifier)
							if current != nil && !current.identified && current.iid == iid {
								log.Printf("Received no identify from droplet %s in 2 minutes, starting delete..", identifier)
								current.delete(true)
							}
						}()
					}
				}()
				break loop
			}
		}
	case payloadActionDelete:
		var data PayloadDeleteData
		err := json.Unmarshal(payload.Data, &data)
		if err != nil {
			log.Printf("Could not unmarshal droplet delete data: %s.\n", err.Error())
			return
		}
		droplet := droplets.get(data.Identifier)
		if droplet == nil {
			log.Printf("Received request to delete invalid droplet: %s.\n", payload.Data)
		} else {
			go droplet.delete(false)
		}
	case payloadActionIdentify:
		var data PayloadDroplet
		err := json.Unmarshal(payload.Data, &data)
		if err != nil {
			log.Printf("Could not unmarshal droplet identify data: %s.\n", err.Error())
			return
		}
		droplet := droplets.get(payload.Sender)
		if droplet == nil {
			log.Printf("Received request to identify invalid droplet: %s.\n", data.Identifier)
		} else {
			droplet.identified = true
			log.Printf("Droplet %s identified, port: %v.\n", droplet.identifier, droplet.port)
		}
	case payloadActionQuery:
		data := &PayloadQueryData{
			Droplets: make([]*PayloadDroplet, 0),
		}
		droplets.forAllDroplets(func(droplet *droplet) {
			if !droplet.identified {
				return
			}
			data.Droplets = append(data.Droplets, droplet.toPayloadEntity())
		})
		bytes, err := json.Marshal(data)
		if err != nil {
			log.Printf("Could not marshal droplet query data: %s.\n", err.Error())
			return
		}
		payloadSend(&Payload{
			Action: payloadActionQuery,
			Sender: payloadSenderHandler,
			Data:   bytes,
			Token:  config.Token,
		})
	}

}
