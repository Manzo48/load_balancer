package main

import (
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "Hello from mock server 1")
    })

    log.Println("Mock server 1 running on :9001")
    log.Fatal(http.ListenAndServe(":9001", nil))
}
