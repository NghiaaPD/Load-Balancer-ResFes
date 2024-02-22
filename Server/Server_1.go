package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, that is server 1 !")
	})

	fmt.Println("Server starting on port 3200...")
	if err := http.ListenAndServe(":3200", nil); err != nil {
		fmt.Println("Error", err)
	}
}
