package main

import (
	"context"
	"os"
	"testing"

	"google.golang.org/genai"
)

// Integration test against the test database; needs DATABASE_URL (and DB_SSH for the tunnel).
func TestCartaTools(t *testing.T) {
	loadDotEnv(".env")
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("sin DATABASE_URL")
	}
	ctx := context.Background()
	db, tunnel, err := openCarta(ctx, url, os.Getenv("DB_SSH"), env("DB_SSH_KEY", "~/.ssh/tunel_pruebas_sisopia"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := runCartaTool(ctx, db, &genai.FunctionCall{Name: "buscar_carta", Args: map[string]any{"texto": "WHISKY"}})
	rows, ok := r["output"].([]map[string]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("buscar_carta whisky: %v", r)
	}
	for _, row := range rows {
		if p, _ := row["precio"].(float64); p <= 0 {
			t.Errorf("sin precio: %v", row)
		}
	}

	// Accent-insensitive: "cafe" finds "Cafés Fríos", filtered to one branch.
	r = runCartaTool(ctx, db, &genai.FunctionCall{Name: "buscar_carta", Args: map[string]any{"texto": "cafe", "sucursal": "sisa"}})
	rows, _ = r["output"].([]map[string]any)
	if len(rows) == 0 {
		t.Fatalf("buscar_carta cafe/sisa: %v", r)
	}
	for _, row := range rows {
		if row["sucursal"] != "SISA Coffee & Wine" {
			t.Errorf("sucursal equivocada: %v", row)
		}
	}

	// Word search: "ron 12" finds the 12-year rums, and "ron" alone must not match "Corona".
	r = runCartaTool(ctx, db, &genai.FunctionCall{Name: "buscar_carta", Args: map[string]any{"texto": "flor de caña 12", "sucursal": "bar"}})
	rows, _ = r["output"].([]map[string]any)
	if len(rows) == 0 || rows[0]["nombre"] != "Flor de caña Centenario 12 años" {
		t.Fatalf("flor de caña 12: %v", r)
	}
	r = runCartaTool(ctx, db, &genai.FunctionCall{Name: "buscar_carta", Args: map[string]any{"texto": "ron"}})
	rows, _ = r["output"].([]map[string]any)
	for _, row := range rows {
		if row["nombre"] == "Corona" {
			t.Errorf("ron coincide con Corona: %v", rows)
		}
	}

	r = runCartaTool(ctx, db, &genai.FunctionCall{Name: "categorias"})
	if rows, _ := r["output"].([]map[string]any); len(rows) == 0 {
		t.Fatalf("categorias: %v", r)
	}

	// A dead SSH session must not break the next query: new connections reopen it.
	if tunnel != nil {
		tunnel.close()
		db.Reset() // drop pooled connections, as a dead session does
		r = runCartaTool(ctx, db, &genai.FunctionCall{Name: "categorias", Args: map[string]any{"sucursal": "bar"}})
		if rows, _ := r["output"].([]map[string]any); len(rows) == 0 {
			t.Fatalf("tras cerrar el túnel: %v", r)
		}
	}

	if r := runCartaTool(ctx, db, &genai.FunctionCall{Name: "borrar_todo"}); r["error"] == nil {
		t.Errorf("tool desconocida sin error: %v", r)
	}
}
