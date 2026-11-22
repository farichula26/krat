package main

import "testing"

func TestCatalog(t *testing.T) {
	if ms := catalog("lfm3-omni:latest", false); len(ms) != 1 || ms[0].Provider != "local" {
		t.Fatalf("without a key only the local model should exist, got %+v", ms)
	}
	ms := catalog("lfm3-omni:latest", true)
	if len(ms) != 3 || ms[1].Provider != "gemini" || ms[1].Default != defaultGeminiVoice {
		t.Fatalf("unexpected catalog with key: %+v", ms)
	}
}

func TestModelLookup(t *testing.T) {
	s := &server{localModel: "lfm3-omni:latest", geminiKey: "k"}
	for id, want := range map[string]string{
		"gemini-3.8-live": "gemini", "lfm3-omni:latest": "local", "": "local", "nope": "local",
	} {
		if got := s.model(id).Provider; got != want {
			t.Errorf("model(%q) = %s, want %s", id, got, want)
		}
	}
	// Without a key a Gemini id must not reach the Live API.
	if got := (&server{localModel: "x"}).model("gemini-3.8-live").Provider; got != "local" {
		t.Errorf("gemini id without key resolved to %s", got)
	}
}
