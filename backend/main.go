package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

type config struct {
	Port, GeminiKey, GeminiModel, DatabaseURL, DBSSH, DBSSHKey, RedisURL string
}

type server struct {
	geminiKey   string
	geminiModel string        // Live model id from .env; never sent to the browser
	carta       *redis.Client // nil: krat gets no menu tools
}

func main() {
	loadDotEnv(".env")
	cfg := config{
		Port:        env("PORT", "8080"),
		GeminiKey:   env("GEMINI_API_KEY", ""),
		GeminiModel: env("GEMINI_MODEL", "gemini-3.8-live"),
		DatabaseURL: env("DATABASE_URL", ""),
		DBSSH:       env("DB_SSH", ""),
		DBSSHKey:    env("DB_SSH_KEY", "~/.ssh/tunel_pruebas_sisopia"),
		RedisURL:    env("REDIS_URL", "redis://127.0.0.1:6379/0"),
	}
	if cfg.GeminiKey == "" {
		log.Fatal("falta GEMINI_API_KEY en backend/.env")
	}
	ctx := context.Background()
	s := &server{geminiKey: cfg.GeminiKey, geminiModel: cfg.GeminiModel}
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/voices", s.handleVoices)

	log.Printf("backend en http://localhost:%s (modelo %s, carta: %v)", cfg.Port, cfg.GeminiModel, s.carta != nil)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// loadDotEnv sets KEY=VALUE lines from path. It overrides the shell on purpose: a global
// variable left over from another project would silently win otherwise.
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
