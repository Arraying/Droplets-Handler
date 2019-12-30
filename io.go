package main

import (
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
)

// ioFileExists checks if a file exists.
func ioFileExists(path string) bool {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return true
	}
	return false
}

// ioCommand executes a shell command.
func ioCommand(command string, arguments ...string) error {
	cmd := exec.Command(command, arguments...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error executing command %s with arguments %v: %s\n", command, arguments, err.Error())
	}
	log.Printf("Execute output (%d): %s\n", len(out), strings.TrimSpace(string(out)))
	return err
}

// ioFreePort finds a random free port.
func ioFreePort() (port int, err error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return
	}
	listener.Close()
	port = listener.Addr().(*net.TCPAddr).Port
	return
}
