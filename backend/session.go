package main

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const maxHistory = 40 // messages (20 turns), text only

type inMsg struct {
	Type  string `json:"type"`
	ID    int    `json:"id"`
	Audio string `json:"audio"`
	Image string `json:"image"`
	Voice string `json:"voice"`
}

type outMsg struct {
	Type    string `json:"type"`
	ID      int    `json:"id,omitempty"`
	Seq     int    `json:"seq"`
	Text    string `json:"text,omitempty"`
	Data    string `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

type session struct {
	srv  *server
	conn *websocket.Conn
	mu   sync.Mutex // serializes turns so history stays ordered
	hist []*schema.Message
}

func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	// The Vite dev proxy forwards the browser's localhost:5173 Origin.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"localhost:*", "127.0.0.1:*"}})
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(16 << 20) // a minute of 16 kHz WAV + a JPEG, base64'd, is ~3 MB

	ctx := r.Context()
	if m := s.model(r.URL.Query().Get("model")); m.Provider == "gemini" {
		log.Printf("ws: conectado %s (%s)", r.RemoteAddr, m.ID)
		defer log.Printf("ws: desconectado %s", r.RemoteAddr)
		s.handleLive(ctx, c, m.ID, r.URL.Query().Get("voice"))
		return
	}
	sess := &session{srv: s, conn: c}
	log.Printf("ws: conectado %s", r.RemoteAddr)
	defer log.Printf("ws: desconectado %s", r.RemoteAddr)
	go func() {
		select {
		case <-s.ready:
			sess.send(ctx, outMsg{Type: "ready"})
		case <-ctx.Done():
		}
	}()

	cancel := context.CancelFunc(func() {})
	defer func() { cancel() }()
	for {
		var m inMsg
		if err := wsjson.Read(ctx, c, &m); err != nil {
			return
		}
		switch m.Type {
		case "interrupt":
			log.Printf("ws: interrupción")
			cancel()
		case "turn":
			if _, err := base64.StdEncoding.DecodeString(m.Audio); err != nil || m.Audio == "" {
				sess.send(ctx, outMsg{Type: "error", ID: m.ID, Message: "audio inválido"})
				continue
			}
			if m.Voice == "" {
				m.Voice = defaultVoice
			}
			cancel()
			tctx, c := context.WithCancel(ctx)
			cancel = c
			go sess.turn(tctx, m)
		}
	}
}

func (s *session) send(ctx context.Context, m outMsg) {
	if err := wsjson.Write(ctx, s.conn, m); err != nil && ctx.Err() == nil {
		log.Printf("ws write: %v", err)
	}
}

func (s *session) turn(ctx context.Context, m inMsg) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		return
	}
	start := time.Now()
	log.Printf("turno %d: audio %d KB, imagen %d KB, voz %s", m.ID, len(m.Audio)*3/4/1024, len(m.Image)*3/4/1024, m.Voice)

	// TTS runs sentence by sentence, in order, while the model keeps generating.
	sentences := make(chan string, 32)
	ttsDone := make(chan struct{})
	go func() {
		defer close(ttsDone)
		seq := 0
		for snt := range sentences {
			if ctx.Err() != nil {
				continue
			}
			err := s.srv.tts.SpeechStream(ctx, snt, m.Voice, func(pcm []byte) {
				s.send(ctx, outMsg{Type: "audio", ID: m.ID, Seq: seq, Data: base64.StdEncoding.EncodeToString(pcm)})
				seq++
			})
			if err != nil && ctx.Err() == nil {
				log.Printf("tts: %v", err)
			}
		}
	}()

	var sp splitter
	reply, err := s.srv.agent.Reply(ctx, s.hist, m.Audio, m.Image, func(d string) {
		s.send(ctx, outMsg{Type: "delta", ID: m.ID, Text: d})
		for _, x := range sp.Push(d) {
			sentences <- x
		}
	})
	for _, x := range sp.Flush() {
		sentences <- x
	}

	// Transcribing only after the reply is generated: Ollama shares one model, and running it
	// in parallel delayed krat's answer by ~1 s. Your caption arrives a bit later instead.
	userText := make(chan string, 1)
	go func() {
		t, err := s.srv.agent.Transcribe(ctx, m.Audio)
		if err != nil && ctx.Err() == nil {
			log.Printf("transcribe: %v", err)
		}
		if t != "" {
			s.send(ctx, outMsg{Type: "transcript", ID: m.ID, Text: t})
		}
		userText <- t
	}()

	close(sentences)
	<-ttsDone
	said := <-userText

	if err != nil && ctx.Err() == nil {
		log.Printf("reply: %v", err)
		s.send(ctx, outMsg{Type: "error", ID: m.ID, Message: "el modelo no respondió"})
		return
	}
	if said == "" {
		said = "(audio)"
	}
	if reply != "" { // keep interrupted partial replies too: the user heard part of them
		s.hist = append(s.hist, schema.UserMessage(said), schema.AssistantMessage(reply, nil))
		if len(s.hist) > maxHistory {
			s.hist = s.hist[len(s.hist)-maxHistory:]
		}
	}
	log.Printf("turno %d: %s en %s · tú: %q · krat: %q", m.ID, map[bool]string{true: "interrumpido", false: "terminado"}[ctx.Err() != nil], time.Since(start).Round(time.Millisecond), said, reply)
	if ctx.Err() == nil {
		s.send(ctx, outMsg{Type: "done", ID: m.ID})
	}
}
