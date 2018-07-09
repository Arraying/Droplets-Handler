package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

type (
	// Config represents the main config file.
	Config struct {
		Redis struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Auth     string `json:"auth"`
			Database int    `json:"database"`
		} `json:"redis"`
		TemplatesDir string `json:"templates-dir"`
		TargetDir    string `json:"target-dir"`
		Token        string `json:"token"`
	}
)

const (
	configFile   = "config.json"
	templateFile = "template.json"
	lockFile     = "droplets.lock"
)

var (
	config    Config
	templates = make([]*Template, 0)
)

// main is the entry point of the program.
func main() {
	log.Println("Checking OS compatibility...")
	if runtime.GOOS != "linux" {
		log.Println("Droplets unable to run on non-linux operating systems.")
		return
	}
	log.Println("Operating system compatible. #LinuxMasterrace.")
	if !initiateLock() {
		log.Println("Droplet lock already exists, perhaps another handler is running?")
		return
	}
	log.Println("Loading configuration...")
	err := loadData(configFile, &config)
	if err != nil {
		panic(err)
	}
	if !config.isValid() {
		log.Println("Configuration is invalid.")
		return
	}
	config.handleDirs()
	log.Println("Loaded configuration.")
	log.Println("Loading templates...")
	var localTemplates []*Template
	err = loadData(templateFile, &localTemplates)
	if err != nil {
		panic(err)
	}
	for _, template := range localTemplates {
		if template.isValid() {
			if template.containsFiles() {
				log.Printf("Registered template %s.\n", template.Name)
				templates = append(templates, template)
			} else {
				log.Printf("Template %s does not contain all required files.\n", template.Name)
			}
		} else {
			log.Printf("Template %s is invalid.\n", template.Name)
		}
	}
	log.Println("Template loading completed.")
	log.Println("Connecting to Redis...")
	err = connectRedis()
	if err != nil {
		panic(err)
	}
	go payloadReceive()
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			droplets.forAllDroplets(func(droplet *droplet) {
				log.Printf("Reported registered droplet %s (identified: %t).\n", droplet.identifier, droplet.identified)
			})
		}
	}()
	keepalive := make(chan os.Signal, 1)
	signal.Notify(keepalive, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-keepalive
	terminate()
}

// isValid checks the validity of a config.
func (c *Config) isValid() bool {
	return c.Redis.Host != "" && c.Redis.Port != 0 && c.TemplatesDir != "" && c.TargetDir != "" && c.Token != ""
}

// handleDirs appends the directory separator to path variables.
func (c *Config) handleDirs() {
	c.TemplatesDir = appendSlash(c.TemplatesDir)
	c.TargetDir = appendSlash(c.TargetDir)
}

// toRedisString creates a Redis connection string.
func (c *Config) toRedisString() string {
	return c.Redis.Host + ":" + strconv.Itoa(c.Redis.Port)
}

// terminate terminates everything.
func terminate() {
	droplets.forAllDroplets(func(droplet *droplet) {
		droplet.delete(true)
	})
	conns.close()
	removeLock()
}
