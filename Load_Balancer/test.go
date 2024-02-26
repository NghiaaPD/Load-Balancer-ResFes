package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func checkServer(serverURL string) bool {
	conn, err := net.DialTimeout("tcp", serverURL, time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func main() {
	serverURL := "localhost:3200,localhost:3201,localhost:3202"

	servers := strings.Split(serverURL, ",")

	for _, server := range servers {
		if checkServer(server) {
			fmt.Println(server, "is alive")
		} else {
			fmt.Println(server, "is not alive")
		}
	}
}
