package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/genai"
)

// cartaInstruction is appended to krat's prompt when the menu database is configured.
const cartaInstruction = `
Atiendes a clientes de un grupo de restaurantes y los ayudas a elegir de la carta.
Para hablar de productos y precios usa siempre las tools categorias y buscar_carta; nunca inventes productos ni precios.
Si no sabes en qué sucursal está el cliente, pregúntaselo (con categorias sabes cuáles hay) o dedúcelo por lo que pide.
Di los precios en soles. No puedes registrar pedidos: si quiere pedir, dile que lo confirme con el personal.
Si el cliente ya te dijo su sucursal, pásala siempre en el parámetro sucursal.
La carta no indica el tipo de licor: un ron aparece por su marca (Flor de Caña, Cartavio, Zacapa). Si buscas un tipo y no sale, busca por la categoría (por ejemplo Destilados por copa o Shots) y usa lo que sabes de las marcas.
Si una tool devuelve error, no la repitas: dile al cliente que ahora no puedes consultar la carta.`

func strParam(desc string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeString, Description: desc}
}

// Typed Parameters, not ParametersJsonSchema: Gemini 2.5 native audio ignored the JSON schema and invented "query".
func params(p map[string]*genai.Schema) *genai.Schema {
	return &genai.Schema{Type: genai.TypeObject, Properties: p}
}

var cartaTools = []*genai.FunctionDeclaration{
	{
		Name:        "categorias",
		Description: "Lista las sucursales con carta y sus categorías, con cuántos productos tiene cada una.",
		Parameters: params(map[string]*genai.Schema{
			"sucursal": strParam("Parte del nombre de la sucursal. Vacío: todas."),
		}),
	},
	{
		Name:        "buscar_carta",
		Description: "Busca productos de la carta con su precio en soles y la sucursal donde se venden. Máximo 20 resultados.",
		Parameters: params(map[string]*genai.Schema{
			"texto":     strParam("Palabras a buscar en el nombre, la categoría o la descripción (por ejemplo whisky, pizza, café, flor de caña 12). Primero salen los productos que coinciden con más palabras."),
			"categoria": strParam("Parte del nombre de la categoría."),
			"sucursal":  strParam("Parte del nombre de la sucursal. Vacío: todas."),
		}),
	},
}

// The menu lives in Redis as one JSON key, copied from Postgres every cartaSync:
// the tools read it locally in under a millisecond and keep working when the tunnel is down.
const (
	cartaKey  = "carta"
	cartaSync = 60 * time.Second
)

type producto struct {
	Sucursal    string   `json:"sucursal"`
	Nombre      string   `json:"nombre"`
	Categoria   string   `json:"categoria"`
	Descripcion string   `json:"descripcion"`
	Precio      *float64 `json:"precio"` // null when the product has no price
}

type categoriaFila struct {
	Sucursal  string `json:"sucursal"`
	Categoria string `json:"categoria"`
	Productos int    `json:"productos"`
}

const syncSQL = `SELECT s.nombre AS sucursal, p.nombre, p.categoria_venta AS categoria, coalesce(p.descripcion, '') AS descripcion,
  -- the price lists differ by channel, but today every channel has the same price
  (SELECT min(pi.precio) FROM ventas_precioitem pi WHERE pi.producto_venta_id = p.id)::float8 AS precio
FROM ventas_productoventa p
JOIN ventas_productoventa_sucursales ps ON ps.productoventa_id = p.id
JOIN core_sucursal s ON s.id = ps.sucursal_id
WHERE p.activo AND s.activa AND s.en_carta_digital`

// syncCarta copies the menu into Redis now and every cartaSync, until ctx ends.
func syncCarta(ctx context.Context, db *pgxpool.Pool, rdb *redis.Client) {
	last := -1
	for t := time.NewTicker(cartaSync); ; {
		if n, err := syncOnce(ctx, db, rdb); err != nil {
			log.Printf("carta: sync falló, se usa la copia de Redis: %v", err)
		} else if n != last {
			log.Printf("carta: %d productos sincronizados", n)
			last = n
		}
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

func syncOnce(ctx context.Context, db *pgxpool.Pool, rdb *redis.Client) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second) // first round opens the SSH tunnel
	defer cancel()
	rows, err := db.Query(ctx, syncSQL)
	if err != nil {
		return 0, err
	}
	carta, err := pgx.CollectRows(rows, pgx.RowToStructByName[producto])
	if err != nil {
		return 0, err
	}
	if len(carta) == 0 {
		return 0, fmt.Errorf("la consulta no devolvió productos; no se pisa la copia")
	}
	b, err := json.Marshal(carta)
	if err != nil {
		return 0, err
	}
	return len(carta), rdb.Set(ctx, cartaKey, b, 0).Err()
}

// foldText lowercases and strips Spanish accents, like the old SQL translate().
var foldText = func() func(string) string {
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")
	return func(s string) string { return r.Replace(strings.ToLower(s)) }
}()

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// buscar ranks products by how many words of texto start a word of the product
// ("ron" must not match "Corona"; "flor de caña 12" puts the right bottle first).
func buscar(carta []producto, texto, categoria, sucursal string) []producto {
	var words []string
	for _, w := range nonWord.Split(foldText(texto), -1) {
		if w != "" {
			words = append(words, " "+w)
		}
	}
	categoria, sucursal = foldText(categoria), foldText(sucursal)
	type hit struct {
		p producto
		n int
	}
	var hits []hit
	for _, p := range carta {
		if !strings.Contains(foldText(p.Categoria), categoria) || !strings.Contains(foldText(p.Sucursal), sucursal) {
			continue
		}
		hay := " " + nonWord.ReplaceAllString(foldText(p.Nombre+" "+p.Categoria+" "+p.Descripcion), " ")
		n := 0
		for _, w := range words {
			if strings.Contains(hay, w) {
				n++
			}
		}
		if n > 0 || len(words) == 0 {
			hits = append(hits, hit{p, n})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int {
		return cmp.Or(b.n-a.n, cmp.Compare(a.p.Sucursal, b.p.Sucursal), cmp.Compare(a.p.Categoria, b.p.Categoria), cmp.Compare(a.p.Nombre, b.p.Nombre))
	})
	out := []producto{}
	for _, h := range hits[:min(len(hits), 20)] {
		out = append(out, h.p)
	}
	return out
}

// categorias counts products per branch and category.
func categorias(carta []producto, sucursal string) []categoriaFila {
	sucursal = foldText(sucursal)
	out := []categoriaFila{}
	for _, p := range carta {
		if !strings.Contains(foldText(p.Sucursal), sucursal) {
			continue
		}
		i := slices.IndexFunc(out, func(c categoriaFila) bool { return c.Sucursal == p.Sucursal && c.Categoria == p.Categoria })
		if i < 0 {
			out = append(out, categoriaFila{p.Sucursal, p.Categoria, 0})
			i = len(out) - 1
		}
		out[i].Productos++
	}
	slices.SortFunc(out, func(a, b categoriaFila) int {
		return cmp.Or(cmp.Compare(a.Sucursal, b.Sucursal), cmp.Compare(a.Categoria, b.Categoria))
	})
	return out
}

// loadCarta reads the Redis copy of the menu.
func loadCarta(ctx context.Context, rdb *redis.Client) ([]producto, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	b, err := rdb.Get(ctx, cartaKey).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("aún no hay copia de la carta en Redis")
	}
	if err != nil {
		return nil, err
	}
	var carta []producto
	return carta, json.Unmarshal(b, &carta)
}

// sucursales lists each branch name once, sorted.
func sucursales(carta []producto) []string {
	var out []string
	for _, p := range carta {
		if !slices.Contains(out, p.Sucursal) {
			out = append(out, p.Sucursal)
		}
	}
	slices.Sort(out)
	return out
}

// cartaPrompt adds the branch names to cartaInstruction: without them Gemini took "estoy en el bar"
// for any bar and kept asking for the name instead of calling categorias.
func cartaPrompt(ctx context.Context, rdb *redis.Client) string {
	carta, err := loadCarta(ctx, rdb)
	if err != nil {
		log.Printf("carta: prompt sin lista de sucursales: %v", err)
		return cartaInstruction
	}
	return cartaInstruction + "\nLas sucursales son: " + strings.Join(sucursales(carta), ", ") + ". " +
		`Si el cliente nombra una aunque sea de forma genérica ("estoy en el bar", "en la barra"), esa es su sucursal: no le pidas otro nombre.`
}

// runCartaTool executes one of cartaTools against the Redis copy and returns the function response Gemini expects.
func runCartaTool(ctx context.Context, rdb *redis.Client, call *genai.FunctionCall) map[string]any {
	if rdb == nil {
		return map[string]any{"error": "la carta no está configurada"}
	}
	if call.Name != "categorias" && call.Name != "buscar_carta" {
		return map[string]any{"error": fmt.Sprintf("tool desconocida: %s", call.Name)}
	}
	carta, err := loadCarta(ctx, rdb)
	if err != nil {
		return cartaError(err)
	}
	arg := func(k string) string { s, _ := call.Args[k].(string); return s }
	if call.Name == "categorias" {
		return map[string]any{"output": categorias(carta, arg("sucursal"))}
	}
	return map[string]any{"output": buscar(carta, arg("texto"), arg("categoria"), arg("sucursal"))}
}

// cartaError tells the model to stop: without it Gemini retried the same failing call in a loop.
func cartaError(err error) map[string]any {
	return map[string]any{"error": "La carta no está disponible ahora mismo. No vuelvas a llamar a la tool en este turno: " +
		"dile al cliente que ahora no puedes consultar la carta y sigue la conversación. (detalle: " + err.Error() + ")"}
}

// toolSummary is the log line for a tool result: row count or the error.
func toolSummary(r map[string]any) string {
	if e, ok := r["error"]; ok {
		return fmt.Sprint(e)
	}
	return fmt.Sprintf("%d filas", reflect.ValueOf(r["output"]).Len())
}
