package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"google.golang.org/genai"
)

func precio(p float64) *float64 { return &p }

var cartaFixture = []producto{
	{"Bar", "Corona", "Cervezas", "", precio(15)},
	{"Bar", "Flor de caña 4 años", "Destilados por copa", "", precio(18)},
	{"Bar", "Flor de caña Centenario 12 años", "Destilados por copa", "", precio(30)},
	{"Bar", "Cartavio 12 años", "Destilados por copa", "", precio(28)},
	{"Bar", "Chivas 12", "Whisky", "", precio(35)},
	{"SISA Coffee & Wine", "Frappé de vainilla", "Cafés Fríos", "", precio(16)},
	{"SISA Coffee & Wine", "Pizza margarita", "Pizzas", "tomate y albahaca", nil},
	{"Curich", "Café americano", "Cafés", "", precio(8)},
}

func TestBuscar(t *testing.T) {
	names := func(ps []producto) (out []string) {
		for _, p := range ps {
			out = append(out, p.Nombre)
		}
		return
	}
	if got := names(buscar(cartaFixture, "ron", "", "")); len(got) != 0 {
		t.Errorf("ron coincide con %v", got)
	}
	if got := buscar(cartaFixture, "cafe", "", "sisa"); len(got) != 1 || got[0].Nombre != "Frappé de vainilla" {
		t.Errorf("cafe/sisa: %v", names(got))
	}
	if got := buscar(cartaFixture, "flor de caña 12", "", "bar"); got[0].Nombre != "Flor de caña Centenario 12 años" {
		t.Errorf("flor de caña 12: %v", names(got))
	}
	if got := buscar(cartaFixture, "ALBAHACA", "", ""); len(got) != 1 || got[0].Precio != nil {
		t.Errorf("descripción / precio null: %v", got)
	}
	if got := buscar(cartaFixture, "", "destilados", ""); len(got) != 3 {
		t.Errorf("sin texto, por categoría: %v", names(got))
	}
	var many []producto
	for i := range 30 {
		many = append(many, producto{Sucursal: "Bar", Nombre: fmt.Sprintf("Pisco %d", i)})
	}
	if got := buscar(many, "pisco", "", ""); len(got) != 20 {
		t.Errorf("límite 20: %d", len(got))
	}

	cats := categorias(cartaFixture, "BAR")
	if len(cats) != 3 || cats[1] != (categoriaFila{"Bar", "Destilados por copa", 3}) {
		t.Errorf("categorias: %v", cats)
	}
}

// Integration test: syncs the test database into the local Redis, then queries through the tools.
// Needs DATABASE_URL (and DB_SSH for the tunnel) and a running Redis.
func TestCartaTools(t *testing.T) {
	loadDotEnv(".env")
	ctx := context.Background()
	opt, err := redis.ParseURL(env("REDIS_URL", "redis://127.0.0.1:6379/0"))
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("sin Redis: ", err)
	}
	if r := runCartaTool(ctx, rdb, &genai.FunctionCall{Name: "borrar_todo"}); r["error"] == nil {
		t.Errorf("tool desconocida sin error: %v", r)
	}

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("sin DATABASE_URL")
	}
	db, _, err := openCarta(ctx, url, os.Getenv("DB_SSH"), env("DB_SSH_KEY", "~/.ssh/tunel_pruebas_sisopia"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if n, err := syncOnce(ctx, db, rdb); err != nil || n == 0 {
		t.Fatalf("sync: %d productos, %v", n, err)
	}

	tool := func(name string, args map[string]any) []producto {
		r := runCartaTool(ctx, rdb, &genai.FunctionCall{Name: name, Args: args})
		rows, ok := r["output"].([]producto)
		if !ok || len(rows) == 0 {
			t.Fatalf("%s %v: %v", name, args, r)
		}
		return rows
	}
	for _, p := range tool("buscar_carta", map[string]any{"texto": "WHISKY"}) {
		if p.Precio == nil || *p.Precio <= 0 {
			t.Errorf("sin precio: %v", p)
		}
	}
	for _, p := range tool("buscar_carta", map[string]any{"texto": "cafe", "sucursal": "sisa"}) {
		if p.Sucursal != "SISA Coffee & Wine" {
			t.Errorf("sucursal equivocada: %v", p)
		}
	}
	if p := tool("buscar_carta", map[string]any{"texto": "flor de caña 12", "sucursal": "bar"}); p[0].Nombre != "Flor de caña Centenario 12 años" {
		t.Errorf("flor de caña 12: %v", p[0])
	}
	if r := runCartaTool(ctx, rdb, &genai.FunctionCall{Name: "categorias"}); len(r["output"].([]categoriaFila)) == 0 {
		t.Errorf("categorias: %v", r)
	}
}
