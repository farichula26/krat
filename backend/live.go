package main

import (
	"context"
	"encoding/base64"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"google.golang.org/genai"
)

const instruction = `Eres krat, un asistente de voz cálido y curioso que conversa en tiempo real con el usuario.
En cada turno recibes el audio de lo que el usuario acaba de decir y, a veces, una imagen de lo que ve su cámara en ese momento.
Escucha el audio y responde a lo que dice. Nunca lo transcribas ni lo repitas.
Usa la imagen solo cuando sea relevante, por ejemplo si te muestra algo o te pregunta qué ves.
Tú no tienes cuerpo ni manos: lo que aparece en la imagen lo tiene el usuario. Háblale de tú ("tienes", "sostienes", "me enseñas").
Responde en el idioma del usuario (por defecto español), de forma natural y breve: de una a tres frases cortas.
No uses listas, markdown ni emojis, porque tu respuesta se convertirá en voz.`

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
	// Messages the page shows never name the provider.
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
		fail("fatal", "No se pudo preparar la conexión.", err)
		return
	}
	lc := &genai.LiveConnectConfig{
		ResponseModalities:       []genai.Modality{genai.ModalityAudio},
		SystemInstruction:        &genai.Content{Parts: []*genai.Part{{Text: instruction}}},
		SpeechConfig:             &genai.SpeechConfig{VoiceConfig: &genai.VoiceConfig{PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: voice}}},
		InputAudioTranscription:  &genai.AudioTranscriptionConfig{},
		OutputAudioTranscription: &genai.AudioTranscriptionConfig{},
		// Camera frames fill the context in ~2 min and Gemini then ends the session, wiping krat's memory;
		// the sliding window drops the oldest turns instead.
		// shortcut: a connection still ends after ~10 min (GoAway) and the reconnect starts fresh; add SessionResumption if calls get that long.
		ContextWindowCompression: &genai.ContextWindowCompressionConfig{SlidingWindow: &genai.SlidingWindow{}},
		// Low start sensitivity + 300 ms of steady speech: a TV crowd or a shout doesn't start a turn or cut krat.
		// 500 ms of silence ends your turn (default ~800): the low end of Google's recommended 500-800 ms.
		RealtimeInputConfig: &genai.RealtimeInputConfig{AutomaticActivityDetection: &genai.AutomaticActivityDetection{
			StartOfSpeechSensitivity: genai.StartSensitivityLow,
			PrefixPaddingMs:          genai.Ptr[int32](300),
			SilenceDurationMs:        genai.Ptr[int32](500),
		}},
	}
	if s.carta != nil {
		lc.SystemInstruction.Parts[0].Text += cartaPrompt(ctx, s.carta)
		lc.Tools = []*genai.Tool{{FunctionDeclarations: cartaTools}}
	}
	sess, err := client.Live.Connect(ctx, model, lc)
	if err != nil {
		fail("fatal", "No se pudo conectar con krat. Revisa la clave y vuelve a intentarlo.", err)
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
					fail("error", "Se cortó la conexión con krat.", err)
				}
				return
			}
			// Cancellations are ignored: the tools only read.
			if m.ToolCall != nil {
				var res []*genai.FunctionResponse
				for _, fc := range m.ToolCall.FunctionCalls {
					start := time.Now()
					r := runCartaTool(ctx, s.carta, fc)
					log.Printf("live: tool %s(%v) → %s (%v)", fc.Name, fc.Args, toolSummary(r), time.Since(start))
					res = append(res, &genai.FunctionResponse{ID: fc.ID, Name: fc.Name, Response: r})
				}
				if err := sessSend(func() error { return sess.SendToolResponse(genai.LiveToolResponseInput{FunctionResponses: res}) }); err != nil {
					if ctx.Err() == nil {
						fail("error", "Se perdió la conexión con krat.", err)
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
				fail("error", "Se perdió la conexión con krat.", err)
			}
			return
		}
	}
}
