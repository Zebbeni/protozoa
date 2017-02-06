package server

import (
	"io"
	"net/http"

	"golang.org/x/net/websocket"
	"simulation"
	"encoding/json"
)

func init() {
	//http.HandleFunc("/", handler)
	http.Handle("/simulate", websocket.Handler(simulationHandler))
	http.Handle("/", http.FileServer(http.Dir("./client")))
}

func simulationHandler(ws *websocket.Conn) {
	sim := simulation.NewSimulation()
	simJson, _ := json.Marshal(sim)
	io.WriteString(ws, string(simJson[:]))
}
