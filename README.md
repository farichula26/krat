# krat

Agente de voz y visión en tiempo real (estilo Anam.ai) sobre la API Live de Gemini. La web no nombra al proveedor ni recibe el ID del modelo.
Ves tu webcam, hablas sin pulsar nada, le enseñas cosas y te responde hablando. Puedes interrumpirle.

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
DATABASE_URL=postgres://lector_pruebas:<contraseña>@127.0.0.1:55432/inventario   # host visto desde el servidor SSH
DB_SSH=tunel_pruebas@167.86.79.210
DB_SSH_KEY=~/.ssh/tunel_pruebas_sisopia
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

## Crecer: tools, MCP, RAG

Las tools se declaran en `LiveConnectConfig.Tools` y se responden en `handleLive` (`backend/live.go`); `carta.go` es el ejemplo.

## Tests

```bash
cd backend && go vet ./... && go test ./...
cd frontend && npm run check && npm run build
```
