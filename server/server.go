package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {

	http.HandleFunc("/hello", HelloHandler)
	fmt.Println("Server started at port 8881")
	log.Fatal(http.ListenAndServe(":8881", nil))
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, there\n")
}
