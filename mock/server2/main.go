package main

import (
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "Hello from mock server 2")
    })

    log.Println("Mock server 2 running on :9002")
    log.Fatal(http.ListenAndServe(":9002", nil))
}
