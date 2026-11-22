package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type modelInfo struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Provider string   `json:"provider"` // "local": Ollama through Eino; "gemini": Live API
	Voices   []string `json:"voices"`
	Default  string   `json:"default"`
}

// The 30 prebuilt voices of Gemini's native audio models. An unknown name is not an error on
// Google's side (it silently falls back), so the list also validates what the browser sends.
var geminiVoices = []string{
	"Zephyr", "Puck", "Charon", "Kore", "Fenrir", "Leda", "Orus", "Aoede", "Callirrhoe", "Autonoe",
	"Enceladus", "Iapetus", "Umbriel", "Algieba", "Despina", "Erinome", "Algenib", "Rasalgethi",
	"Laomedeia", "Achernar", "Alnilam", "Schedar", "Gacrux", "Pulcherrima", "Achird", "Zubenelgenubi",
	"Vindemiatrix", "Sadachbia", "Sadaltager", "Sulafat",
}

const defaultGeminiVoice = "Kore"

// catalog lists the models krat can talk through. Local voices are filled in by the caller;
// Gemini models only exist when GEMINI_API_KEY is set.
func catalog(localModel string, gemini bool) []modelInfo {
	ms := []modelInfo{{ID: localModel, Label: "lfm3-omni (local)", Provider: "local", Default: defaultVoice}}
	if gemini {
		for _, g := range []struct{ id, label string }{
			{"gemini-3.8-live", "Gemini 3.8 Live"},
			{"gemini-2.5-flash-native-audio-latest", "Gemini 2.5 Flash Native Audio"},
		} {
			ms = append(ms, modelInfo{ID: g.id, Label: g.label, Provider: "gemini", Voices: geminiVoices, Default: defaultGeminiVoice})
		}
	}
	return ms
}

// model returns the catalog entry for id; an empty or unknown id means the local model.
func (s *server) model(id string) modelInfo {
	ms := catalog(s.localModel, s.geminiKey != "")
	for _, m := range ms {
		if m.ID == id {
			return m
		}
	}
	return ms[0]
}

func (s *server) handleModels(w http.ResponseWriter, r *http.Request) {
	ms := catalog(s.localModel, s.geminiKey != "")
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	voices, err := s.tts.Voices(ctx)
	if err != nil || len(voices) == 0 {
		log.Printf("voices: %v", err)
		voices = []string{defaultVoice} // the page still works with the default voice
	}
	ms[0].Voices = voices
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"models": ms})
}
