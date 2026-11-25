package main

import (
	"log"
	"net/http"

	"github.com/coder/websocket"
)

type outMsg struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Data    string `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	// The Vite dev proxy forwards the browser's localhost:5173 Origin.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"localhost:*", "127.0.0.1:*"}})
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(16 << 20) // generous: a camera JPEG base64'd is well under this

	log.Printf("ws: conectado %s (%s)", r.RemoteAddr, s.geminiModel)
	defer log.Printf("ws: desconectado %s", r.RemoteAddr)
	s.handleLive(r.Context(), c, s.geminiModel, r.URL.Query().Get("voice"))
}
