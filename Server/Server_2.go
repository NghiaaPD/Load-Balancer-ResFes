package main

import (
	"fmt"
	"net/http"
)

func main() {
	port := "3201"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, that is server 2 !")
	})

	fmt.Println("Server starting on port " + port + "...")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Println("Error", err)
	}
}
