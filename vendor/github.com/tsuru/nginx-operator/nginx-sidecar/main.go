package main

import (
	"github.com/tsuru/nginx-operator/nginx-sidecar/handlers"
	"log"
	"net/http"
)

const listen = ":59999"

func main() {
	http.HandleFunc("/healthcheck", handlers.HealthcheckHandler)

	log.Fatal(http.ListenAndServe(listen, nil))
}
