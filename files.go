package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
)

type (
	bootVariable struct {
		placeholder string
		value       string
	}
)

// loadData loads data and caches it
func loadData(path string, data interface{}) (err error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(bytes, data)
	return
}

// saveData saves data to the file.
func saveData(path string, data interface{}) (err error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(path, bytes, 0644)
	return
}

// templatePath gets the path for the template file.
func templatePath(template, file string) string {
	return config.TemplatesDir + template + "/" + file
}

// targetPath gets the path for the target file.
func targetPath(identifier, file string) string {
	return config.TargetDir + identifier + "/" + file
}

// fileExists checks whether the specified file exists.
func fileExists(path string) bool {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

// deleteExists deletes the path if it exists.
func deleteExists(path string) (err error) {
	if fileExists(path) {
		err = execute("rm", "-rf", path)
	}
	return
}

// replacecAll replaces all variables
func replaceAll(input string, vars ...bootVariable) string {
	for _, variable := range vars {
		input = strings.Replace(input, variable.placeholder, variable.value, -1)
	}
	return input
}
