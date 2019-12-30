package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type (
	// Config represents the main config file.
	Config struct {
		Redis      ConfigRedis   `json:"redis"`    // The Redis credentials.
		Timings    ConfigTimings `json:"timings"`  // The timings.
		ExternalIP string        `json:"external"` // The external IP of the node. Leave blank if the proxy is also on the same node.
		Token      string        `json:"token"`    // The token, an internal password for Droplets.
	}
	// ConfigRedis represents the Redis database credentials and information.
	ConfigRedis struct {
		Host string // The host, including port.
		Auth string // The auth string.
	}
	// ConfigTimings represents the timings configuration.
	ConfigTimings struct {
		Identify int `json:"identify"` // The time, in seconds, to wait for identifies.
		Destroy  int `json:"destroy"`  // The time, in seconds, before the Droplet will be removed after the destroy payload.
		Notify   int `json:"notify"`   // The interval, in seconds, at which the notification should occur.
	}
)

var (
	config    Config
	templates []Template
	conns     connections
	store     = garage{
		make(map[string]*droplet),
		sync.RWMutex{},
	}
	iidIncrement uint64
)

// Main is the entry point of the program.
func main() {
	log.SetPrefix("Droplets-Handler ")
	if !lockGet() {
		log.Println("Cannot get lock. Is another instance running?")
		return
	}
	log.Println("Loading config.json.")
	bites, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Println("Could not read config.json, does it exist?")
		panic(err)
	}
	err = json.Unmarshal(bites, &config)
	if err != nil {
		log.Println("Could not load config.json, is it a valid JSON object?")
		panic(err)
	}
	log.Println("Loading templates.json.")
	bites, err = ioutil.ReadFile("templates.json")
	if err != nil {
		log.Println("Could not read templates.json, does it exist?")
		panic(err)
	}
	err = json.Unmarshal(bites, &templates)
	if err != nil {
		log.Println("Could not load templates.json, is it a valid JSON array?")
		panic(err)
	}
	log.Println("Connecting to Redis.")
	redisConnect()
	go redisPayloadListen()
	go func() {
		notify := 60
		if config.Timings.Notify > 0 {
			notify = config.Timings.Notify
		}
		for {
			time.Sleep(time.Duration(notify) * time.Second)
			log.Println("-- BEGIN REPORT --")
			store.forEach(func(droplet *droplet) {
				log.Printf("* %s (identified: %t)\n", droplet.identifier, droplet.identified)
			})
			log.Println("-- END REPORT --")
		}
	}()
	keepalive := make(chan os.Signal, 1)
	signal.Notify(keepalive, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-keepalive
	store.forEach(func(droplet *droplet) {
		droplet.destroy(true)
	})
	redisDisconnect()
	lockRemove()
	log.Println("Goodbye.")
}

// lockGet attempts to get a lock for the current Droplet handler.
// That way, multiple handlers cannot run with the same files.
// Returns true if the lock was established, false if there is already a lock.
func lockGet() bool {
	if ioFileExists("droplets.lock") {
		return false
	}
	_, err := os.Create("droplets.lock")
	if err != nil {
		panic(err)
	}
	return true
}

// lockRemove cleans up the lock after it is no longer needed.
func lockRemove() {
	if err := os.Remove("droplets.lock"); err != nil {
		panic(err)
	}
}
