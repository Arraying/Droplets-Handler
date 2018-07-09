package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

// getFreePort gets a free port.
func getFreePort() (port int, err error) {
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

// getOutboundAddress gets the outbound IP address.
// Yes, it's gotten this far.
func getOutboundAddress() string {
	request, err := http.Get("http://checkip.amazonaws.com")
	if err != nil {
		return ""
	}
	defer request.Body.Close()
	ip, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(ip))
}
