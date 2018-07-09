package main

import (
	"encoding/json"
	"log"

	"github.com/gomodule/redigo/redis"
)

type (
	// Payload represents a base Redis payload.
	Payload struct {
		Action string          `json:"a"`
		Sender string          `json:"s"`
		Data   json.RawMessage `json:"d"`
		Token  string          `json:"t"`
	}
	// PayloadCreateData contains the create payload data.
	PayloadCreateData struct {
		Template string `json:"x"`
		Data     string `json:"v"`
	}
	// PayloadDeleteData contains the delete payload data.
	PayloadDeleteData struct {
		Identifier string `json:"i"`
	}
	// PayloadQueryData contains the query payload data.
	PayloadQueryData struct {
		Droplets []*PayloadDroplet `json:"l"`
	}
	// PayloadDroplet represents a droplet representation inside a payload.
	PayloadDroplet struct {
		Identifier string `json:"i"`
		IP         string `json:"h"`
		Port       int    `json:"p"`
		Data       string `json:"v"`
	}
	connections struct {
		regular redis.Conn
		pubSub  *redis.PubSubConn
	}
	initialRedisCommand struct {
		command  string
		argument interface{}
	}
)

const (
	payloadChannel          = "ch_dr"
	payloadActionCreate     = "c"
	payloadActionDelete     = "d"
	payloadActionIdentify   = "i"
	payloadActionQuery      = "q"
	payloadSenderProxy      = "_"
	payloadSenderHandler    = "#"
	payloadSplitIdentifier  = "-"
	payloadSplitAddress     = ":"
	payloadSplitDropletMeta = "@"
	payloadSplitDropletList = ","
)

var (
	conns                connections
	initialRedisCommands = []initialRedisCommand{
		initialRedisCommand{
			command:  "auth",
			argument: &config.Redis.Auth,
		},
		initialRedisCommand{
			command:  "select",
			argument: &config.Redis.Database,
		},
	}
)

// close closes all connections.
func (c *connections) close() {
	c.regular.Close()
	c.pubSub.Close()
}

// connectRedis connects to the Redis server.
func connectRedis() error {
	regular, err1 := redisConnection()
	pubSubRaw, err2 := redisConnection()
	if err1 != nil {
		panic(err1)
	} else if err2 != nil {
		panic(err2)
	}
	pubSub := &redis.PubSubConn{Conn: pubSubRaw}
	pubSub.Subscribe(payloadChannel)
	conns = connections{
		regular: regular,
		pubSub:  pubSub,
	}
	return nil
}

// redisConnection creates a new Redis connection.
func redisConnection() (con redis.Conn, err error) {
	con, err = redis.Dial("tcp", config.toRedisString())
	if err != nil {
		return
	}
	log.Println("New connection to Redis established.")
	for _, command := range []initialRedisCommand{
		initialRedisCommand{
			command:  "auth",
			argument: config.Redis.Auth,
		},
		initialRedisCommand{
			command:  "select",
			argument: config.Redis.Database,
		},
	} {
		log.Printf("Executing command %s with argument %v.\n", command.command, command.argument)
		_, err := con.Do(command.command, command.argument)
		if err != nil {
			return con, err
		}
	}
	log.Println("All Redis commands executed.")
	return
}
