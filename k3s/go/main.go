package main

import (
	"fmt"
	"net/http"
)

func Serve(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "It's working!")
}

func main() {
	http.HandleFunc("/", Serve)
	http.ListenAndServe(":8080", nil)
}
