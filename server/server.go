package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := "8080"
	http.HandleFunc("/hello", HelloHandler)
	fmt.Println("Server started at port " + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Hello, there</h1>\n")
}
