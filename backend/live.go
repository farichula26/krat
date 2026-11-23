package main

import (
	"context"
	"encoding/base64"
	"log"
	"slices"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"google.golang.org/genai"
)

// liveIn is what the browser sends during a Gemini call: continuous 16 kHz PCM and a JPEG about once a second.
type liveIn struct {
	Type  string `json:"type"` // "audio" | "frame"
	Data  string `json:"data"`
	Image string `json:"image"`
}

// handleLive bridges the browser to Gemini's Live API. Google runs the turns, the speech detection
// and the interruptions; this only forwards media and translates the replies.
func (s *server) handleLive(ctx context.Context, c *websocket.Conn, model, voice string) {
	var wmu sync.Mutex // one writer at a time: the receive loop and error paths both send
	send := func(m outMsg) {
		wmu.Lock()
		defer wmu.Unlock()
		if err := wsjson.Write(ctx, c, m); err != nil && ctx.Err() == nil {
			log.Printf("live: ws write: %v", err)
		}
	}
	// kind "fatal" tells the page not to retry (bad key, unknown model); "error" is a cut mid-call.
	fail := func(kind, user string, err error) {
		log.Printf("live: %s: %s", user, strings.ReplaceAll(err.Error(), s.geminiKey, "***"))
		send(outMsg{Type: kind, Message: user})
	}

	if !slices.Contains(geminiVoices, voice) {
		voice = defaultGeminiVoice
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:      s.geminiKey,
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{APIVersion: "v1beta"}, // the Live endpoint is not served under v1
	})
	if err != nil {
		fail("fatal", "No se pudo preparar la conexión con Gemini.", err)
		return
	}
	lc := &genai.LiveConnectConfig{
		ResponseModalities:       []genai.Modality{genai.ModalityAudio},
		SystemInstruction:        &genai.Content{Parts: []*genai.Part{{Text: instruction}}},
		SpeechConfig:             &genai.SpeechConfig{VoiceConfig: &genai.VoiceConfig{PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: voice}}},
		InputAudioTranscription:  &genai.AudioTranscriptionConfig{},
		OutputAudioTranscription: &genai.AudioTranscriptionConfig{},
	}
	if s.db != nil {
		lc.SystemInstruction.Parts[0].Text += cartaInstruction
		lc.Tools = []*genai.Tool{{FunctionDeclarations: cartaTools}}
	}
	sess, err := client.Live.Connect(ctx, model, lc)
	if err != nil {
		fail("fatal", "No se pudo conectar con Gemini. Revisa la clave y vuelve a intentarlo.", err)
		return
	}
	defer sess.Close()
	log.Printf("live: conectado a %s (voz %s)", model, voice)
	send(outMsg{Type: "ready"})

	// genai.Session writes to its websocket without a lock; tool replies and media come from two goroutines.
	var smu sync.Mutex
	sessSend := func(f func() error) error {
		smu.Lock()
		defer smu.Unlock()
		return f()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		defer cancel() // Gemini closed (time limit, network): end the call so the page reconnects
		var you, krat strings.Builder
		newTurn := true
		for {
			m, err := sess.Receive()
			if err != nil {
				if ctx.Err() == nil {
					fail("error", "Gemini cerró la conexión.", err)
				}
				return
			}
			// Cancellations are ignored: the tools only read.
			if m.ToolCall != nil {
				var res []*genai.FunctionResponse
				for _, fc := range m.ToolCall.FunctionCalls {
					r := runCartaTool(ctx, s.db, fc)
					log.Printf("live: tool %s(%v) → %s", fc.Name, fc.Args, toolSummary(r))
					res = append(res, &genai.FunctionResponse{ID: fc.ID, Name: fc.Name, Response: r})
				}
				if err := sessSend(func() error { return sess.SendToolResponse(genai.LiveToolResponseInput{FunctionResponses: res}) }); err != nil {
					if ctx.Err() == nil {
						fail("error", "Se perdió la conexión con Gemini.", err)
					}
					return
				}
			}
			sc := m.ServerContent
			if sc == nil {
				continue
			}
			// The page clears its captions when a new exchange starts.
			startTurn := func() {
				if newTurn {
					newTurn = false
					send(outMsg{Type: "newTurn"})
				}
			}
			if t := sc.InputTranscription; t != nil && t.Text != "" {
				startTurn()
				you.WriteString(t.Text)
				send(outMsg{Type: "transcript", Text: t.Text})
			}
			if t := sc.OutputTranscription; t != nil && t.Text != "" {
				startTurn()
				krat.WriteString(t.Text)
				send(outMsg{Type: "delta", Text: t.Text})
			}
			if sc.ModelTurn != nil {
				for _, p := range sc.ModelTurn.Parts {
					// Text parts are the model's thought summaries, not speech.
					if p.InlineData != nil && strings.HasPrefix(p.InlineData.MIMEType, "audio/pcm") {
						send(outMsg{Type: "audio", Data: base64.StdEncoding.EncodeToString(p.InlineData.Data)})
					}
				}
			}
			if sc.Interrupted {
				send(outMsg{Type: "interrupted"})
				newTurn = true
			}
			if sc.TurnComplete {
				log.Printf("live: tú: %q · krat: %q", you.String(), krat.String())
				you.Reset()
				krat.Reset()
				newTurn = true
				send(outMsg{Type: "turnDone"})
			}
		}
	}()

	for {
		var m liveIn
		if err := wsjson.Read(ctx, c, &m); err != nil {
			return
		}
		var in genai.LiveRealtimeInput
		switch m.Type {
		case "audio":
			pcm, err := base64.StdEncoding.DecodeString(m.Data)
			if err != nil || len(pcm) == 0 {
				continue
			}
			in.Audio = &genai.Blob{Data: pcm, MIMEType: "audio/pcm;rate=16000"}
		case "frame":
			img, err := base64.StdEncoding.DecodeString(m.Image)
			if err != nil || len(img) == 0 {
				continue
			}
			in.Video = &genai.Blob{Data: img, MIMEType: "image/jpeg"}
		default:
			continue
		}
		if err := sessSend(func() error { return sess.SendRealtimeInput(in) }); err != nil {
			if ctx.Err() == nil {
				fail("error", "Se perdió la conexión con Gemini.", err)
			}
			return
		}
	}
}
