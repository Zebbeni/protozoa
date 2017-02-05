package server

import (
	"net/http"
	"simulation"
)

func init() {
	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	simulation.RunSimulation(w)
}
