# krat

Agente de voz y visión en tiempo real (estilo Anam.ai) sobre la API Live de Gemini. La web no nombra al proveedor ni recibe el ID del modelo.
Ves tu webcam, hablas sin pulsar nada, le enseñas cosas y te responde hablando. Puedes interrumpirle.

## Requisitos

- Go 1.26+ y Node.js 20+ (npm).
- Una clave de la API de Gemini con acceso a la API Live ([Google AI Studio](https://aistudio.google.com/apikey)).
- Navegador con webcam y micrófono (Chrome, Edge o Safari).
- Opcional, solo para la carta del restaurante: Redis (o Podman/Docker para levantarlo), una base Postgres y, si está tras un servidor SSH, una llave privada con acceso.

## Estructura

```
backend/    Go: WebSocket ↔ Gemini Live, tools de la carta, túnel SSH, sync Postgres → Redis
  main.go     arranque, .env, rutas HTTP
  session.go  /ws: acepta el WebSocket del navegador
  live.go     puente con Gemini Live (prompt, audio, imágenes, tools, transcripciones)
  models.go   las 30 voces y /api/voices
  carta.go    sync Postgres → Redis, tools categorias y buscar_carta
  tunnel.go   túnel SSH propio con keepalive para llegar a Postgres
frontend/   Svelte 5 + Vite + TypeScript: la llamada (micrófono, VAD, cámara, audio, subtítulos)
colab/      Notebook de prueba de un avatar generado (Ditto + Seed-VC) en una T4; experimento aparte
docs/       Especificaciones de diseño; PRODUCT.md y DESIGN.md describen producto e interfaz
```

## Arrancar

```bash
# 1. Backend (Go) — puerto 8080
cd backend
cp .env.example .env   # y pon GEMINI_API_KEY
go run .

# 2. Frontend (Svelte + Vite) — http://localhost:5173
cd frontend
npm install            # solo la primera vez
npm run dev
```

Abre http://localhost:5173, pulsa **Empezar llamada** y acepta micrófono y cámara.
Mejor con auriculares: la cancelación de eco del navegador ayuda, pero no es perfecta.

### Desde otro dispositivo de la red (iPad, móvil)

Safari y Chrome solo permiten micrófono y cámara con HTTPS (o en `localhost`). Para usarlo desde otro equipo, en vez de `npm run dev` lanza:

```bash
cd frontend && npm run dev:lan
```

Vite imprime una URL `Network: https://<ip-del-portátil>:5173/`; ábrela en el iPad. El certificado es autofirmado: en Safari toca **Mostrar detalles → visitar este sitio web**. El backend no cambia, porque Vite le reenvía `/ws` y `/api`.

`backend/.env` tiene prioridad sobre las variables del shell (a propósito: una variable global de otro proyecto no se cuela).

| Variable | Por defecto |
|---|---|
| `PORT` | `8080` |
| `GEMINI_API_KEY` | **obligatoria**: sin ella el backend no arranca |
| `GEMINI_MODEL` | `gemini-3.8-live` |
| `DATABASE_URL` | vacía: krat no consulta la carta |
| `DB_SSH` | vacía: conexión directa a `DATABASE_URL`, sin túnel |
| `DB_SSH_KEY` | `~/.ssh/tunel_pruebas_sisopia` |
| `REDIS_URL` | `redis://127.0.0.1:6379/0`: copia local de la carta |

## Modelo

krat habla siempre a través de un modelo de la API Live de Gemini; se cambia con `GEMINI_MODEL` en `backend/.env` y reiniciando el backend. La clave y el modelo se quedan en el backend: la página solo pide las voces (`/api/voices`) y abre `/ws?voice=…`, y ningún texto visible nombra al proveedor.

- El micrófono se envía de forma continua (PCM de 16 kHz) y la cámara a un fotograma por segundo. Google factura por minuto de conexión: cuelga al terminar.
- La voz se elige antes de llamar y se fija al conectar; se recuerda entre visitas.
- Una clave inválida o un modelo no disponible muestra un aviso y cuelga sin reintentar (el error real queda en el log del backend). Un corte en plena llamada sí se reintenta.
- Si preguntas la hora, krat responde en hora UTC, no en la tuya.

## Carta del restaurante

Con `DATABASE_URL` en `backend/.env`, krat atiende como mozo: consulta la carta en la base de datos `inventario` (solo lectura, datos de prueba) y da los precios reales. La base de datos solo es accesible por SSH, y el backend abre ese túnel él mismo; no hace falta un `ssh -L` aparte:

```
DATABASE_URL=postgres://USUARIO:CONTRASEÑA@127.0.0.1:5432/NOMBRE_BD   # host visto desde el servidor SSH
DB_SSH=usuario@host-ssh
DB_SSH_KEY=~/.ssh/id_ed25519
```

krat no consulta el Postgres en cada pregunta. El backend copia la carta entera a un Redis local cada 60 s, y las tools la leen de ahí en menos de 1 ms. Si el Postgres o el túnel se caen, krat sigue con la última copia, incluso después de reiniciar el backend. Para arrancar Redis (una vez; se queda solo en `127.0.0.1`):

```bash
podman run -d --name krat-redis --network host --userns=keep-id --restart unless-stopped \
  -v "$PWD/data/redis":/data docker.io/library/redis:8-alpine \
  redis-server --bind 127.0.0.1 --port 6379 --appendonly yes
podman exec krat-redis redis-cli GET carta   # ver la copia
```

- El túnel se abre en el primer sync. Un keepalive cada 15 s lo cierra si el servidor deja de contestar (suspensión del portátil, cortes de red), y el siguiente sync abre uno nuevo.
- La llave del servidor se verifica contra `~/.ssh/known_hosts`: conéctate una vez con `ssh` para registrarla.

krat dispone de dos tools (`backend/carta.go`), que buscan sobre la copia de Redis:

| Tool | Devuelve |
|---|---|
| `categorias(sucursal?)` | Sucursales con carta y sus categorías |
| `buscar_carta(texto?, categoria?, sucursal?)` | Hasta 20 productos con sucursal, categoría, descripción y precio en soles |

- Las búsquedas ignoran mayúsculas y tildes y van por palabras: cada palabra debe empezar una palabra del producto («ron» no encuentra «Corona»). Primero salen los productos que coinciden con más palabras.
- La carta no indica el tipo de licor (un ron aparece como Flor de Caña o Cartavio). Si una búsqueda por tipo no da nada, krat revisa la categoría y usa lo que sabe de las marcas.
- Si el cliente no dice la sucursal, krat la pregunta o la deduce por lo que pide (por ejemplo, el whisky solo está en Bar).
- krat no registra pedidos.
- Cada llamada queda en el log: `live: tool buscar_carta(map[sucursal:Bar texto:whisky]) → 3 filas (1.2ms)`.
- Si un sync falla, queda en el log (`carta: sync falló, se usa la copia de Redis`). Solo si Redis no responde, o aún no tiene copia, la tool devuelve un error que le pide al modelo no reintentar; krat avisa al cliente y la llamada sigue.

## Cómo funciona

```
navegador                                 backend Go                       Gemini Live
 mic ─ Silero VAD ─ PCM 16 kHz ─┐
 webcam ─ 1 JPEG 640px/s ───────┼─ WS {audio,frame} ─▶ handleLive ─────────▶ sesión Live
 altavoz ◀─ PCM 24 kHz ─────────┴─◀ {audio,delta,transcript,interrupted} ◀─┘
```

- **Turnos e interrupciones:** los decide la detección de voz de Google. El VAD del navegador solo sirve para el estado en pantalla y para transmitir el micrófono.
- **Subtítulos:** las transcripciones de entrada y salida de la sesión Live.
- **Memoria:** la ventana deslizante de contexto de la sesión; una reconexión empieza de cero.

## Protocolo WebSocket (`/ws?voice=Kore`)

Navegador → backend (JSON):

| Mensaje | Contenido |
|---|---|
| `{type:"audio", data}` | PCM 16 kHz, 16 bits, mono, en base64; continuo mientras el micrófono esté activo |
| `{type:"frame", image}` | JPEG de 640 px de ancho en base64, uno por segundo |

Backend → navegador:

| Mensaje | Significado |
|---|---|
| `ready` | La sesión Live está conectada |
| `audio` (`data`) | Trozo de voz de krat, PCM 24 kHz 16 bits en base64; se reproduce sin cortes |
| `transcript` / `delta` (`text`) | Subtítulo de lo que dijiste / de lo que responde krat |
| `newTurn`, `turnDone`, `interrupted` | Límites de turno; `interrupted` vacía el audio pendiente |
| `error` / `fatal` (`message`) | Corte a mitad de llamada (la página reintenta con espera creciente, máx. 8 s) / rechazo definitivo (clave o modelo inválidos; no reintenta) |

Otros endpoints: `GET /api/voices` (lista de voces y la de por defecto) y `GET /healthz` (`ok`).

## Detalles de la sesión con Gemini

- El prompt de krat (`instruction` en `backend/live.go`) lo pide breve (1-3 frases), sin listas ni markdown, en el idioma del usuario.
- Detección de turnos de Google: inicio de voz con sensibilidad baja, 300 ms de voz sostenida para empezar y 500 ms de silencio para cerrar el turno.
- Ventana deslizante de contexto: sin ella, los fotogramas llenan el contexto en ~2 min y Gemini cierra la sesión. Aun así, la conexión termina a los ~10 min y la reconexión empieza sin memoria.
- En el navegador, el umbral del VAD (Silero, local, sin CDN) sube de 0,5 a 0,85 mientras krat habla, para que el eco de su propia voz no lo interrumpa.
- Los mensajes de error que ve el usuario nunca nombran al proveedor, y la clave se oculta de los logs.

## Seguridad

- `backend/.env` está en `.gitignore`; nunca lo subas. `backend/.env.example` solo lleva marcadores.
- La clave de Gemini y el ID del modelo no llegan al navegador.
- El WebSocket solo acepta orígenes `localhost` y `127.0.0.1`. Con `npm run dev:lan` Vite hace de proxy, así que el backend sigue viendo `localhost`. Para desplegarlo en otro dominio hay que cambiar `OriginPatterns` en `backend/session.go` y añadir autenticación (hoy no hay: cualquiera que llegue al backend gasta tu cuota de Gemini).
- La base de datos se usa solo en lectura: usa un usuario de Postgres que solo pueda hacer `SELECT`.

## Esquema de la carta esperado

El sync (`syncSQL` en `backend/carta.go`) lee estas tablas de Postgres; adáptalo si tu esquema difiere: `ventas_productoventa` (nombre, categoria_venta, descripcion, activo), `ventas_productoventa_sucursales`, `core_sucursal` (nombre, activa, en_carta_digital) y `ventas_precioitem` (precio). Se toma el precio mínimo por producto.

## Solución de problemas

| Síntoma | Causa probable |
|---|---|
| El backend dice `falta GEMINI_API_KEY` | No hay `backend/.env` o la clave está vacía |
| Aviso «No se pudo conectar con krat» | Clave inválida o `GEMINI_MODEL` no disponible; el detalle está en el log del backend |
| `known_hosts … conéctate una vez con ssh` | Haz `ssh usuario@host` una vez para registrar la llave del servidor |
| `carta: sync falló` | Postgres o el túnel caídos; krat sigue con la última copia de Redis |
| La tool devuelve «aún no hay copia de la carta» | Redis vacío: espera al primer sync (o revisa `REDIS_URL`) |
| Otro dispositivo no deja usar micrófono/cámara | Necesita HTTPS: usa `npm run dev:lan` |
| krat se interrumpe solo | Eco: usa auriculares |

## Crecer: tools, MCP, RAG

Las tools se declaran en `LiveConnectConfig.Tools` y se responden en `handleLive` (`backend/live.go`); `carta.go` es el ejemplo.

## Tests

```bash
cd backend && go vet ./... && go test ./...
cd frontend && npm run check && npm run build
```
