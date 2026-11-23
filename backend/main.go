package main

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultVoice = "female-spanish"

type config struct {
	OllamaURL, Model, Port, GeminiKey, DatabaseURL, DBSSH, DBSSHKey, RedisURL string
}

type server struct {
	agent      *Agent
	tts        speaker
	ready      chan struct{} // closed once the model is loaded on the Ollama side
	localModel string
	geminiKey  string        // empty: the Gemini models are not offered
	carta      *redis.Client // nil: Gemini gets no menu tools
}

func main() {
	loadDotEnv(".env")
	cfg := config{
		OllamaURL:   env("OLLAMA_URL", "http://localhost:11434"),
		Model:       env("OLLAMA_MODEL", "lfm3-omni:latest"),
		Port:        env("PORT", "8080"),
		GeminiKey:   env("GEMINI_API_KEY", ""),
		DatabaseURL: env("DATABASE_URL", ""),
		DBSSH:       env("DB_SSH", ""),
		DBSSHKey:    env("DB_SSH_KEY", "~/.ssh/tunel_pruebas_sisopia"),
		RedisURL:    env("REDIS_URL", "redis://127.0.0.1:6379/0"),
	}
	ctx := context.Background()
	agent, err := newAgent(ctx, cfg)
	if err != nil {
		log.Fatalf("agent: %v", err)
	}
	s := &server{agent: agent, tts: speaker{cfg.OllamaURL, cfg.Model}, ready: make(chan struct{}), localModel: cfg.Model, geminiKey: cfg.GeminiKey}
	if cfg.DatabaseURL != "" {
		// Lazy: the pool connects on first query, so a network failure only fails that sync round.
		db, _, err := openCarta(ctx, cfg.DatabaseURL, cfg.DBSSH, cfg.DBSSHKey)
		if err != nil {
			log.Fatalf("DATABASE_URL: %v", err)
		}
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			log.Fatalf("REDIS_URL: %v", err)
		}
		s.carta = redis.NewClient(opt)
		go syncCarta(ctx, db, s.carta)
	}
	go s.warmup(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-s.ready:
			w.Write([]byte("ok"))
		default:
			http.Error(w, "warming up", http.StatusServiceUnavailable)
		}
	})
	mux.HandleFunc("GET /api/models", s.handleModels)

	log.Printf("backend en http://localhost:%s (modelo %s en %s, Gemini: %v, carta: %v)", cfg.Port, cfg.Model, cfg.OllamaURL, cfg.GeminiKey != "", s.carta != nil)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}

// warmup loads the model on the Ollama side; the first cold call takes ~30 s.
func (s *server) warmup(ctx context.Context) {
	defer close(s.ready)
	start := time.Now()
	wav, err := s.tts.Speech(ctx, "Hola.", defaultVoice)
	if err == nil {
		_, err = s.agent.Transcribe(ctx, base64.StdEncoding.EncodeToString(wav))
	}
	if err != nil {
		log.Printf("warmup falló (¿OLLAMA_URL correcta?): %v", err)
		return
	}
	log.Printf("modelo listo en %s", time.Since(start).Round(time.Millisecond))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// loadDotEnv sets KEY=VALUE lines from path. It overrides the shell on purpose: a global
// OLLAMA_URL pointing at another server is common and would silently win otherwise.
func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" || strings.HasPrefix(k, "#") {
			continue
		}
		os.Setenv(k, strings.Trim(strings.TrimSpace(v), `"'`))
	}
}
