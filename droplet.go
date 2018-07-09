package main

import (
	"encoding/json"
	"log"
	"os/exec"
	"sync/atomic"
	"time"
)

// create creates a new droplet.
func (t *Template) create(data string) (drop *droplet, err error) {
	log.Printf("Starting the generation of a droplet of type %s.\n", t.Name)
	identifier := generateDropletIdentifier(t.Name)
	port, err := getFreePort()
	if err != nil {
		log.Println("Obtaining free port error.")
		return
	}
	address := getOutboundAddress()
	log.Printf("Using outbound IP address %s.\n", address)
	log.Printf("Using free port %d.\n", port)
	drop = &droplet{
		identifier: identifier,
		ip:         address,
		port:       port,
		template:   t,
		iid:        atomic.AddUint64(&internalDropletHandlerID, 1),
	}
	deleteTerminal(identifier)
	target := targetPath(identifier, "")
	template := templatePath(t.Name, "")
	if err = deleteExists(targetPath(identifier, "")); err != nil {
		return
	}
	err = execute("cp", "-r", template, target)
	if err != nil {
		return
	}
	for _, template := range requiredFiles {
		path := targetPath(identifier, template.name)
		switch template.handlerUID {
		case handlerUIDBoot:
			template.handler(path, identifier, data, t)
		case handlerUIDLogs:
			template.handler(path)
		case handlerUIDConfig:
			template.handler(path, identifier, data)
		case handlerUIDServer:
			template.handler(path, address, port)
		}
	}
	droplets.put(identifier, drop)
	return
}

// boot boots a droplet.
func (d *droplet) boot() error {
	if !droplets.contains(d.identifier) {
		return errDropletDeleted
	}
	path := targetPath(d.identifier, "boot.sh")
	err := execute("chmod", "+x", path)
	err = executeSpecial(func(cmd *exec.Cmd) {
		cmd.Dir = targetPath(d.identifier, "")
	}, path)
	return err
}

// delete deletes a droplet.
func (d *droplet) delete(payload bool) error {
	if !droplets.contains(d.identifier) {
		return errDropletDeleted
	}
	if payload {
		data, err := json.Marshal(d.toPayloadEntity())
		if err != nil {
			return err
		}
		payloadSend(&Payload{
			Action: payloadActionDelete,
			Sender: payloadSenderHandler,
			Data:   data,
			Token:  config.Token,
		})
	}
	log.Printf("Deleting droplet %s in 15 seconds.\n", d.identifier)
	time.Sleep(15 * time.Second)
	deleteTerminal(d.identifier)
	droplets.remove(d.identifier)
	err := deleteExists(targetPath(d.identifier, ""))
	if err != nil {
		return err
	}
	log.Printf("Deleted droplet %s.\n", d.identifier)
	return nil
}

// toPayloadEntity converts the droplet to a payload entity.
func (d *droplet) toPayloadEntity() *PayloadDroplet {
	return &PayloadDroplet{
		Identifier: d.identifier,
		IP:         d.ip,
		Port:       d.port,
		Data:       d.data,
	}
}
