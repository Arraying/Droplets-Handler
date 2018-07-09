package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"sync"
)

type (
	// Template represents a droplet template.
	Template struct {
		Name      string `json:"name"`
		MinMemory int    `json:"min-memory"`
		MaxMemory int    `json:"max-memory"`
	}
	dropletMap struct {
		droplets map[string]*droplet
		mutex    sync.Mutex
	}
	droplet struct {
		identifier string
		ip         string
		port       int
		data       string
		template   *Template
		identified bool
		iid        uint64
	}
	dropletFile struct {
		name       string
		required   bool
		handler    func(string, ...interface{})
		handlerUID int
	}
)

const (
	pluginName       = "Droplets"
	statusOnline     = "online"
	statusOffline    = "offline"
	handlerUIDBoot   = 1
	handlerUIDLogs   = 2
	handlerUIDConfig = 3
	handlerUIDServer = 4
	filePlugins      = "plugins/"
	fileSpigot       = "spigot.jar"
)

var (
	droplets = dropletMap{
		droplets: make(map[string]*droplet),
	}
	internalDropletHandlerID uint64
	requiredFiles            = []dropletFile{
		dropletFile{
			name:     "boot.sh",
			required: true,
			handler: func(path string, args ...interface{}) {
				if len(args) < 2 {
					log.Println("Expected 2 arguments, found none.")
					return
				}
				identifier := args[0].(string)
				data := args[1].(string)
				template := args[2].(*Template)
				bytes, err := ioutil.ReadFile(path)
				if err != nil {
					log.Printf("Could not edit file %s: %s.\n", path, err.Error())
					return
				}
				str := string(bytes)
				variables := []bootVariable{
					bootVariable{
						placeholder: "IDENTIFIER",
						value:       identifier,
					},
					bootVariable{
						placeholder: "MEMORY_MAX",
						value:       strconv.Itoa(template.MaxMemory),
					},
					bootVariable{
						placeholder: "MEMORY_MIN",
						value:       strconv.Itoa(template.MinMemory),
					},
					bootVariable{
						placeholder: "SPIGOT",
						value:       targetPath(identifier, fileSpigot),
					},
					bootVariable{
						placeholder: "DATA",
						value:       data,
					},
				}
				str = replaceAll(str, variables...)
				log.Printf("Modified boot script: %s.\n", str)
				err = ioutil.WriteFile(path, []byte(str), 0644)
				if err != nil {
					log.Printf("Could not save file %s: %s.\n", path, err.Error())
				}
			},
			handlerUID: handlerUIDBoot,
		},
		dropletFile{
			name:     "logs/",
			required: false,
			handler: func(path string, args ...interface{}) {
				err := os.RemoveAll(path)
				if err != nil {
					log.Printf("Could not delete %s directory: %s.\n", path, err.Error())
				}
			},
			handlerUID: handlerUIDLogs,
		},
		dropletFile{
			name:     filePlugins + pluginName + ".jar",
			required: true,
		},
		dropletFile{
			name:     filePlugins + pluginName + "/",
			required: true,
		},
		dropletFile{
			name:     filePlugins + pluginName + "/config.json",
			required: true,
			handler: func(path string, args ...interface{}) {
				identifier := args[0].(string)
				data := args[1].(string)
				var dataMap map[string]interface{}
				err := loadData(path, &dataMap)
				if err != nil {
					log.Printf("Could not load config %s.\n", path)
					return
				}
				dataMap["identifier"] = identifier
				if data != "" {
					dataMap["data"] = data
				}
				err = saveData(path, &dataMap)
				if err != nil {
					log.Printf("Could not save config %s.\n", path)
				}
			},
			handlerUID: handlerUIDConfig,
		},
		dropletFile{
			name:     "server.properties",
			required: true,
			handler: func(path string, args ...interface{}) {
				ip := args[0].(string)
				port := args[1].(int)
				contents, err := ioutil.ReadFile(path)
				if err != nil {
					log.Printf("Could not read %s: %s.\n", path, err.Error())
					return
				}
				contents = bytes.Replace(contents, []byte("IP"), []byte(ip), -1)
				contents = bytes.Replace(contents, []byte("PORT"), []byte(strconv.Itoa(port)), -1)
				err = ioutil.WriteFile(path, contents, 0644)
				if err != nil {
					log.Printf("Could not write %s: %s.\n", path, err.Error())
				}
			},
			handlerUID: handlerUIDServer,
		},
		dropletFile{
			name:     fileSpigot,
			required: true,
		},
	}
	errDropletDeleted = errors.New("droplet no longer exists")
)

// isValid checks the validity of a template.
func (t *Template) isValid() bool {
	return t.Name != "" && t.MinMemory > 0 && t.MaxMemory > 0 && t.MinMemory <= t.MaxMemory
}

// containsFiles checks if the template contains all required files.
func (t *Template) containsFiles() bool {
	for i := 0; i < len(requiredFiles); i++ {
		requiredFile := &requiredFiles[i]
		path := templatePath(t.Name, requiredFile.name)
		if !fileExists(path) && requiredFile.required {
			log.Printf("Missing file %s for template %s.\n", path, t.Name)
			return false
		}
	}
	return true
}
