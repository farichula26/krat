package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Speech and voice listing are not covered by Eino, so they go straight to Ollama's HTTP API.
type speaker struct {
	baseURL, model string
}

var ttsCleaner = strings.NewReplacer("*", "", "#", "", "`", "")

// Speech returns a 24 kHz mono WAV for text.
func (s speaker) Speech(ctx context.Context, text, voice string) ([]byte, error) {
	body, _ := json.Marshal(map[string]string{"model": s.model, "input": ttsCleaner.Replace(text), "voice": voice})
	var out []byte
	err := s.post(ctx, "/v1/audio/speech", body, func(r io.Reader) (err error) { out, err = io.ReadAll(r); return })
	return out, err
}

// SpeechStream calls onPCM with chunks of 16-bit little-endian mono 24 kHz PCM as Ollama generates
// them, so playback starts long before the sentence is fully synthesized.
func (s speaker) SpeechStream(ctx context.Context, text, voice string, onPCM func([]byte)) error {
	body, _ := json.Marshal(map[string]string{"model": s.model, "input": ttsCleaner.Replace(text), "voice": voice, "response_format": "pcm"})
	return s.post(ctx, "/v1/audio/speech", body, func(r io.Reader) error {
		buf := make([]byte, 9600) // 0.2 s
		for {
			n, err := io.ReadFull(r, buf)
			if n -= n % 2; n > 0 {
				onPCM(buf[:n])
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	})
}

func (s speaker) Voices(ctx context.Context) ([]string, error) {
	body, _ := json.Marshal(map[string]string{"model": s.model})
	var show struct {
		Voices []string `json:"voices"`
	}
	err := s.post(ctx, "/api/show", body, func(r io.Reader) error { return json.NewDecoder(r).Decode(&show) })
	return show.Voices, err
}

func (s speaker) post(ctx context.Context, path string, body []byte, read func(io.Reader) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("ollama %s: %s: %s", path, res.Status, msg)
	}
	return read(res.Body)
}
