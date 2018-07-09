package main

import (
	"log"
	"os/exec"
	"strings"
)

// execute executes a command in the shell.
func execute(command string, args ...string) error {
	return executeSpecial(nil, command, args...)
}

// executeSpecial executes a command in the shell, but allows the command argument to be modified.
func executeSpecial(handler func(*exec.Cmd), command string, args ...string) error {
	cmd := exec.Command(command, args...)
	if handler != nil {
		handler(cmd)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error executing command %s with arguments %v: %s\n", command, args, err.Error())
	}
	log.Printf("Execute output (%d): %s\n", len(out), strings.TrimSpace(string(out)))
	return err
}

// deleteTerminal deletes the terminal.
func deleteTerminal(identifier string) {
	execute("tmux", "kill-session", "-t", identifier)
}
