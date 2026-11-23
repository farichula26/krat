# krat consulta la carta (tool de base de datos, solo Gemini Live)

## Context
Farid quiere que krat atienda clientes como lo hace hoy el avatar «Kathy» del sistema de restaurantes (BD `inventario`): recomendar productos y dar precios reales. La BD es de prueba, de solo lectura, y solo se llega a ella por un túnel SSH. Decisiones del usuario:
- **Tools fijas:** nada de SQL libre.
- **Solo en Gemini Live:** el modo local (lfm3-omni) se queda como está.
- **Sucursal:** la dice el cliente; si no la dice, krat pregunta o la deduce por lo que pide.

Lo que hay en la BD:
- Sucursales con carta: SISA Coffee & Wine (154 productos), Bar (132) y Curich (70). Limanesas tiene 0 productos; Almacén Central está inactiva.
- Productos: `ventas_productoventa` (nombre, categoria_venta, descripcion, activo).
- Relación producto–sucursal: `ventas_productoventa_sucursales`.
- Precios: `ventas_precioitem` → `ventas_listaprecio` (por canal). Hoy no hay ningún producto con precios distintos entre canales, así que se usa `min(precio)`.
- No existe la extensión `unaccent`: se usa `translate(lower(x),'áéíóúüñ','aeiouun')`.
- Crear pedidos queda fuera porque la BD es de solo lectura.

## Approaches (considerados)
1. **Tools fijas con SQL parametrizado ← elegido.** Fiable, seguro frente al cliente y rápido (las queries ya están probadas contra la BD).
2. SQL libre escrito por el modelo: frágil, choca con columnas protegidas y es inapropiado delante de un cliente.
3. Carta en el prompt, sin tool: simple, pero no es lo pedido.

## Cambios

### 0. Llave y secretos
- Mover `tunel_pruebas_sisopia` de la raíz del repo (hoy sin rastrear y **sin ignorar**, con permisos 664) a `~/.ssh/tunel_pruebas_sisopia` y aplicarle `chmod 600`.
- Añadir `tunel_pruebas_sisopia` a `/.gitignore` por si vuelve a caer en el repo.

### 1. Configuración (`backend/main.go`, `backend/.env.example`)
- Nueva variable `DATABASE_URL` (ej. `postgres://lector_pruebas:…@127.0.0.1:55432/inventario`). Se añade a `config` y `server`, siguiendo el patrón de `GeminiKey`.
- **Si está vacía, no se registran tools** y todo sigue como hoy.
- En `main()`: `pgxpool.New` (dependencia nueva `github.com/jackc/pgx/v5`, no hay driver de Postgres en la stdlib).
- No se hace ping bloqueante al arrancar: si el túnel está caído, la tool devuelve un error y Gemini lo dice.
- El túnel lo abre el usuario a mano con `ssh -i ~/.ssh/tunel_pruebas_sisopia -N -L 55432:127.0.0.1:55432 tunel_pruebas@167.86.79.210`, documentado en el README.
- shortcut: túnel manual; si molesta, se puede abrir desde Go con `x/crypto/ssh` (ya es dependencia indirecta).

### 2. Nuevo `backend/carta.go`
- `cartaTools []*genai.FunctionDeclaration` con `ParametersJsonSchema` (todos los parámetros son strings opcionales):
  - `categorias(sucursal?)`: devuelve filas `{sucursal, categoria, productos}`. Sin sucursal, lista las de todas, y así el modelo sabe qué sucursales existen para preguntarle al cliente.
  - `buscar_carta(texto?, categoria?, sucursal?)`: devuelve hasta 20 filas `{sucursal, nombre, categoria, descripcion, precio}`. `texto` busca en nombre, categoría y descripción, sin distinguir mayúsculas ni tildes. Sin sucursal busca en todas, y la columna `sucursal` permite deducirla (whisky → solo Bar, pizzas → SISA).
- `func runCartaTool(ctx, db *pgxpool.Pool, call *genai.FunctionCall) map[string]any`:
  - Timeout de 5 s.
  - Devuelve `{"output": filas}` o `{"error": "..."}`. Un nombre de tool desconocido también da error.
  - Filtro común: `p.activo AND s.activa AND s.en_carta_digital`.
  - Los valores siempre van como parámetros `$1…`, nunca concatenados.
- Las dos queries ya se validaron en el prototipo (whisky → Whisky Sour S/34, Chivas 12 S/35, JW Black S/36; sisa+pizza → 6 pizzas).

### 3. `backend/live.go` (`handleLive`)
- Si hay BD, añadir a `LiveConnectConfig`:
  - `Tools: []*genai.Tool{{FunctionDeclarations: cartaTools}}`
  - La instrucción `instruction + cartaInstruction`. Esta última dice: «Atiendes a clientes de un restaurante. Para productos y precios usa siempre las tools y nunca inventes precios. Si no sabes la sucursal, pregunta (usa `categorias` para saber cuáles hay) o dedúcela por lo que pide. Di los precios en soles».
- **Mutex para escribir en la sesión:** `genai.Session` escribe en un websocket de gorilla sin lock. Si `SendToolResponse` (goroutine de recepción) y `SendRealtimeInput` (bucle principal) escriben a la vez, el programa entra en pánico. Un `smu sync.Mutex` envuelve ambos envíos.
- En el bucle de recepción, antes de `sc := m.ServerContent`:
  - Si llega `m.ToolCall`, por cada `FunctionCall` se ejecuta `runCartaTool`, se registra en el log `tool <name>(<args>) → n filas` y se responde con `SendToolResponse` usando el mismo `ID` y `Name`.
  - `ToolCallCancellation` se ignora: las tools son de solo lectura y no tienen efectos.

### 4. Docs
- README:
  - Fila `DATABASE_URL` en la tabla de variables.
  - Subsección «Carta (Gemini)» con el comando del túnel y las dos tools.
  - Actualizar «Crecer: tools…».
- Guardar este diseño como `docs/superpowers/specs/2026-11-22-carta-tool-design.md`.
- Sin cambios en el frontend: no hay UI nueva, así que no aplican impeccable ni frontend-design.

### 5. Test
- `backend/carta_test.go`: test de integración que hace `t.Skip` si no hay `DATABASE_URL`.
  - `buscar_carta{texto:"whisky"}` devuelve filas, todas con precio > 0 y sucursal "Bar".
  - Una tool desconocida devuelve `error`.

## Verificación
1. `cd backend && go vet ./... && go test ./...`, con el túnel abierto y `DATABASE_URL` en `.env`, para que corra el test de integración.
2. `go run .`, y en el log debe aparecer que la BD está configurada.
3. Con agent-browser:
   - Abrir el frontend con Chrome usando `--use-fake-device-for-media-stream --use-file-for-fake-audio-capture=<wav>` y un WAV con «¿Tienes whisky?».
   - Elegir Gemini y llamar. Comprobar en el log del backend `tool buscar_carta(...) → n filas` y que el subtítulo de krat cite precios reales.
   - Si el audio falso no funciona, Farid hace una llamada real y revisamos el log juntos.

## Revisión tras la primera prueba real (22/11, noche)
- **Túnel zombi:** el `ssh -L` manual seguía escuchando con la sesión muerta y todas las consultas agotaban el timeout. Ahora el backend abre el túnel (`backend/tunnel.go`, con `x/crypto/ssh` y verificación por `known_hosts`): keepalive cada 15 s, reconexión en el siguiente dial, sin puerto local. Variables `DB_SSH` y `DB_SSH_KEY`.
- **Gemini 2.5 inventaba `query`:** con `ParametersJsonSchema` lo ignoraba. Se cambió a `Parameters` tipado (`genai.Schema`), y ahora usa `texto`.
- **Bucle de reintentos:** el error de la tool pide no reintentar y avisar al cliente. Comprobado: una sola llamada.
- **Búsqueda por palabras:** la carta no tiene «ron» (son Flor de Caña, Cartavio, Zacapa) y «ron» coincidía con «Corona». Cada palabra debe empezar una palabra del producto, y el orden es por número de coincidencias. La instrucción le dice al modelo que, si no encuentra un tipo de licor, revise la categoría.
