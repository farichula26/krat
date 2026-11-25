package main

import (
	"encoding/json"
	"net/http"
)

// The 30 prebuilt voices of Gemini's native audio models. An unknown name is not an error on
// Google's side (it silently falls back), so the list also validates what the browser sends.
var geminiVoices = []string{
	"Zephyr", "Puck", "Charon", "Kore", "Fenrir", "Leda", "Orus", "Aoede", "Callirrhoe", "Autonoe",
	"Enceladus", "Iapetus", "Umbriel", "Algieba", "Despina", "Erinome", "Algenib", "Rasalgethi",
	"Laomedeia", "Achernar", "Alnilam", "Schedar", "Gacrux", "Pulcherrima", "Achird", "Zubenelgenubi",
	"Vindemiatrix", "Sadachbia", "Sadaltager", "Sulafat",
}

const defaultGeminiVoice = "Kore"

// handleVoices lists the voices only: the model id stays on the server.
func (s *server) handleVoices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"voices": geminiVoices, "default": defaultGeminiVoice})
}
