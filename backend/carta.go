package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// fold lowercases and strips Spanish accents; the database has no unaccent extension.
func fold(x string) string { return "translate(lower(" + x + "),'áéíóúüñ','aeiouun')" }

func like(col, param string) string { return fold(col) + " LIKE '%' || " + fold(param) + " || '%'" }

const cartaFrom = `FROM ventas_productoventa p
JOIN ventas_productoventa_sucursales ps ON ps.productoventa_id = p.id
JOIN core_sucursal s ON s.id = ps.sucursal_id
`

var (
	categoriasSQL = `SELECT s.nombre AS sucursal, p.categoria_venta AS categoria, count(*)::int AS productos ` + cartaFrom +
		`WHERE p.activo AND s.activa AND s.en_carta_digital AND ` + like("s.nombre", "$1::text") + `
GROUP BY 1, 2 ORDER BY 1, 2`

	// Each word of texto must start a word of the product ("ron" must not match "Corona");
	// products matching more words come first, so "flor de caña 12" ranks the right bottle on top.
	buscarSQL = `SELECT sucursal, nombre, categoria, descripcion, precio FROM (
  SELECT s.nombre AS sucursal, p.nombre, p.categoria_venta AS categoria, coalesce(p.descripcion, '') AS descripcion,
    -- the price lists differ by channel, but today every channel has the same price
    (SELECT min(pi.precio) FROM ventas_precioitem pi WHERE pi.producto_venta_id = p.id)::float8 AS precio,
    (SELECT count(*) FROM unnest(regexp_split_to_array(` + fold("$1::text") + `, '[^a-z0-9]+')) w
      WHERE w <> '' AND ' ' || regexp_replace(` + fold("p.nombre || ' ' || p.categoria_venta || ' ' || coalesce(p.descripcion, '')") + `, '[^a-z0-9]+', ' ', 'g') LIKE '% ' || w || '%') AS hits
  ` + cartaFrom + `WHERE p.activo AND s.activa AND s.en_carta_digital
    AND ` + like("p.categoria_venta", "$2::text") + `
    AND ` + like("s.nombre", "$3::text") + `
) x WHERE hits > 0 OR ` + fold("$1::text") + ` !~ '[a-z0-9]'
ORDER BY hits DESC, sucursal, categoria, nombre LIMIT 20`
)

// runCartaTool executes one of cartaTools and returns the function response Gemini expects.
func runCartaTool(ctx context.Context, db *pgxpool.Pool, call *genai.FunctionCall) map[string]any {
	if db == nil {
		return map[string]any{"error": "la carta no está configurada"}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	arg := func(k string) string { s, _ := call.Args[k].(string); return s }

	var rows pgx.Rows
	var err error
	switch call.Name {
	case "categorias":
		rows, err = db.Query(ctx, categoriasSQL, arg("sucursal"))
	case "buscar_carta":
		rows, err = db.Query(ctx, buscarSQL, arg("texto"), arg("categoria"), arg("sucursal"))
	default:
		return map[string]any{"error": fmt.Sprintf("tool desconocida: %s", call.Name)}
	}
	if err != nil {
		return cartaError(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return cartaError(err)
	}
	return map[string]any{"output": out}
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
	return fmt.Sprintf("%d filas", len(r["output"].([]map[string]any)))
}
