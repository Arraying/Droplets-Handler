package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
)

type (
	// Template represents a Droplet template.
	// Templates are to Droplets what classes are to Objects.
	Template struct {
		Name        string   `json:"name"`            // The template name.
		Image       string   `json:"image"`           // The Docker image name.
		RestrictCPU int      `json:"cpu-restriction"` // The CPU restriction on the container.
		RestrictRAM string   `json:"ram-restriction"` // The RAM restriction on the container.
		Networks    []string `json:"networks"`        // A list of networks to connect the container to.
		Mounts      []struct {
			From string `json:"from"` // The location on the host.
			To   string `json:"to"`   // The location on the container.
		} `json:"mounts"` // A list of mounts to mount on the container.
		Flags string `json:"flags"` // Any additional flags.
	}
	// droplet is a droplet instance.
	droplet struct {
		identifier string    // The unique identifier for the Droplet. This can be re-assigned in the lifetime of the handler.
		iid        uint64    // The unique internal identifier for the Droplet. This cannot be re-assigned in the lifetime of the handler.
		template   *Template // The template used for this Droplet.
		identified bool      // Whether or not the container has identified to the handler.
		ip         string    // The IP.
		port       int       // The port.
		data       string    // The data that was specified during the creation.
	}
	// garage acts as a central storage location for all Droplets.
	// It contains the data required to make the map work.
	garage struct {
		droplets map[string]*droplet // A map of Droplet identifier to the actual Droplet.
		lock     sync.RWMutex        // A lock such that concurrency is safely possible. Wear protection, kids.
	}
)

// construct constructs a Droplet from the current template.
func (template *Template) construct(data string) (instance *droplet, err error) {
	identifier := dropletGenerateIdentifier(template.Name)
	port, err := ioFreePort()
	if err != nil {
		log.Println("Error obtaining free port.")
		return
	}
	address := "127.0.0.1"
	if config.ExternalIP != "" {
		address = config.ExternalIP
	}
	log.Printf("Using outbound IP address %s.\n", address)
	log.Printf("Using free port %d.\n", port)
	instance = &droplet{
		identifier: identifier,
		iid:        atomic.AddUint64(&iidIncrement, 1),
		template:   template,
		identified: false,
		ip:         address,
		port:       port,
		data:       data,
	}
	return
}

// creates the Droplet container.
func (droplet *droplet) create() error {
	log.Println("Creating Docker container.")
	command, arguments := dropletGenerateRun(droplet)
	err := ioCommand(command, arguments...)
	if err != nil {
		return err
	}
	log.Println("Attaching networks.")
	for _, network := range droplet.template.Networks {
		err = ioCommand("docker", "network", "connect", network, droplet.identifier)
		if err != nil {
			return err
		}
	}
	return nil
}

// runs the Droplet container.
func (droplet *droplet) run() error {
	log.Println("Starting container.")
	return ioCommand("docker", "container", "start", droplet.identifier)
}

// destroys the Droplet container.
func (droplet *droplet) destroy(payload bool) error {
	log.Print("Forcefully removing container.")
	return ioCommand("docker", "container", "rm", "-f", droplet.identifier)
}

// toPayloadData converts the Droplet into a PayloadDroplet.
func (droplet *droplet) toPayloadData() *PayloadDroplet {
	return &PayloadDroplet{
		Identifier: droplet.identifier,
		IP:         droplet.ip,
		Port:       droplet.port,
		Data:       droplet.data,
	}
}

// get gets a Droplet by identifier.
func (garage *garage) get(identifier string) *droplet {
	garage.lock.RLock()
	defer garage.lock.RUnlock()
	return garage.droplets[identifier]
}

// contains checks if a Droplet with a certain identifier exists.
func (garage *garage) contains(identifier string) bool {
	garage.lock.RLock()
	defer garage.lock.RUnlock()
	_, contains := garage.droplets[identifier]
	return contains
}

// put adds a new Droplet to the storage.
func (garage *garage) put(droplet *droplet) {
	garage.lock.Lock()
	defer garage.lock.Unlock()
	garage.droplets[droplet.identifier] = droplet
}

// remove removes a Droplet from the storage.
func (garage *garage) remove(identifier string) {
	garage.lock.Lock()
	defer garage.lock.Unlock()
	delete(garage.droplets, identifier)
}

// forEach executes a lambda for each registered Droplet.
func (garage *garage) forEach(lambda func(*droplet)) {
	for _, droplet := range garage.droplets {
		lambda(droplet)
	}
}

// dropletGenerateRun generates the run command syntax.
func dropletGenerateRun(droplet *droplet) (command string, arguments []string) {
	command = "docker"
	arguments = []string{
		"create",
		"--name",
		droplet.identifier,
		"-d",
		"-e",
		"DROPLET_IDENTIFIER=" + droplet.identifier,
		"DROPLET_IP=" + droplet.ip,
		"DROPLET_DATA=" + droplet.data,
	}
	arguments = append(arguments, strings.Split(droplet.template.Flags, " ")...)
	if droplet.template.RestrictCPU > 0 {
		arguments = append(arguments, fmt.Sprintf("--cpus=%d", droplet.template.RestrictCPU))
	}
	if droplet.template.RestrictRAM != "" {
		arguments = append(arguments, fmt.Sprintf("--memory=%s", droplet.template.RestrictRAM))
	}
	for _, mount := range droplet.template.Mounts {
		arguments = append(arguments, "-v", fmt.Sprintf("%s:%s", mount.From, mount.To))
	}
	arguments = append(arguments, droplet.template.Image)
	return
}

// dropletGenerateIdentifier generates an available Droplet identifier.
func dropletGenerateIdentifier(template string) string {
	id := 0
	contains := true
	for contains {
		id++
		contains = store.contains(dropletFormatIdentifier(template, id))
	}
	return dropletFormatIdentifier(template, id)
}

// dropletFormatIdentifier formats a template name and incremental ID to form an identifier.
func dropletFormatIdentifier(template string, id int) string {
	return fmt.Sprintf("%s-%d", template, id)
}
