package main

import (
	"fmt"
	"os"
	"strings"
)

// initiateLock handles the droplet lock. Return value determines whether app is allowed to run.
func initiateLock() bool {
	if !fileExists(lockFile) {
		_, err := os.Create(lockFile)
		if err != nil {
			panic(err)
		}
		return true
	}
	return false
}

// removeLock removes the lock.
func removeLock() {
	if err := os.Remove(lockFile); err != nil {
		panic(err)
	}
}

// appends a slash to the string, if it does not exist.
func appendSlash(origin string) string {
	if !strings.HasSuffix(origin, "/") {
		origin = origin + "/"
	}
	return origin
}

// generateDropletIdentifier generates the next available droplet identifier.
func generateDropletIdentifier(template string) string {
	id := 0
	contains := true
	for contains {
		id++
		contains = droplets.contains(formatDropletIdentifier(template, id))
	}
	return formatDropletIdentifier(template, id)
}

// formatDropeltIdentifier creates a droplet identifier from the template and ID.
func formatDropletIdentifier(template string, id int) string {
	return fmt.Sprintf("%s%s%d", template, payloadSplitIdentifier, id)
}
