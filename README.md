# krat

Agente de voz y visión en tiempo real (estilo Anam.ai) sobre un modelo local: `lfm3-omni` en Ollama.
Ves tu webcam, hablas sin pulsar nada, le enseñas cosas y te responde hablando. Puedes interrumpirle.

## Arrancar

```bash
# 1. Backend (Go + Eino) — puerto 8080
cd backend
cp .env.example .env   # y pon OLLAMA_URL con la URL actual del túnel
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

`backend/.env` tiene prioridad sobre las variables del shell (a propósito: si tienes un `OLLAMA_URL` global apuntando a otro servidor, no se cuela).

| Variable | Por defecto |
|---|---|
| `OLLAMA_URL` | `http://localhost:11434` |
| `OLLAMA_MODEL` | `lfm3-omni:latest` |
| `PORT` | `8080` |
| `GEMINI_API_KEY` | vacía: solo aparece el modelo local |
| `DATABASE_URL` | vacía: Gemini no consulta la carta |
| `DB_SSH` | vacía: conexión directa a `DATABASE_URL`, sin túnel |
| `DB_SSH_KEY` | `~/.ssh/tunel_pruebas_sisopia` |
| `REDIS_URL` | `redis://127.0.0.1:6379/0`: copia local de la carta |

## Elegir modelo

Antes de llamar, el selector junto a «Empezar llamada» permite elegir el modelo. Se recuerda entre visitas, igual que la voz de cada modelo.

| Modelo | Dónde corre | Cómo habla |
|---|---|---|
| **lfm3-omni (local)** | Tu Ollama, con el agente de Eino | Un clip por turno; ver «Cómo funciona» |
| **Gemini 3.8 Live**, **Gemini 2.5 Flash Native Audio** | API Live de Google | Audio y vídeo en continuo; Google decide los turnos, las interrupciones y la voz |

- Los modelos de Gemini solo aparecen si `backend/.env` tiene `GEMINI_API_KEY`. La clave se queda en el backend y nunca llega al navegador.
- Con Gemini el micrófono se envía de forma continua (PCM de 16 kHz) y la cámara a un fotograma por segundo. Google factura por minuto de conexión: cuelga al terminar.
- Con Gemini la voz se fija al empezar la llamada. Se reutiliza el mismo prompt de krat.
- Una clave inválida o un modelo no disponible muestra un aviso y cuelga sin reintentar. Un corte en plena llamada sí se reintenta.
- Si preguntas la hora, Gemini responde en hora UTC, no en la tuya.

## Carta del restaurante (solo Gemini)

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

Gemini dispone de dos tools (`backend/carta.go`), que buscan sobre la copia de Redis:

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

> Esta sección describe el modo local (lfm3-omni).

```
navegador                                backend Go                         Ollama
 mic ─ Silero VAD ─ WAV ─┐
 webcam ─ 1 JPEG 640px ──┼─ WS {turn} ─▶ agente Eino (ADK) ─ stream ─────▶ /api/chat
                         │              └ frases ─ TTS en streaming ─────▶ /v1/audio/speech
 altavoz ◀─ PCM 24 kHz ──┼─◀ {audio}
 hablas mientras habla ──┴─ WS {interrupt} ─▶ cancela el turno
```

- **Turnos:** el VAD (Silero, en el navegador) detecta cuándo empiezas y terminas de hablar y envía el clip más un fotograma de la cámara.
- **Respuesta:** el agente (`backend/agent.go`, `adk.ChatModelAgent`) genera texto en streaming. Se corta en frases (`sentence.go`) y cada frase se sintetiza y se reenvía en trozos de PCM mientras Ollama la genera.
- **Audio de entrada:** el adaptador de Ollama de Eino solo acepta partes de imagen, así que el WAV viaja como una "imagen". Ollama detecta el formato por el contenido.
- **Subtítulo de lo que dijiste:** sale de una segunda llamada al mismo modelo con el prompt `Perform ASR.`, lanzada cuando ya terminó la respuesta para no retrasarla.
- **Historial:** se guarda como texto, los últimos 20 turnos. Solo el turno actual lleva audio e imagen.

Latencia medida (modelo ya caliente, a través del túnel): unos 3,4 s desde que terminas de hablar hasta que krat empieza a sonar. El primer arranque del modelo tarda unos 30 s; el backend lo precalienta al iniciar.

## Crecer: tools, MCP, RAG

En Gemini Live, las tools se declaran en `LiveConnectConfig.Tools` y se responden en `handleLive` (`backend/live.go`); `carta.go` es el ejemplo. En el modo local todo pasa por `newAgent` en `backend/agent.go`:
- **Tools:** añade tools de Eino en `ToolsConfig` del `ChatModelAgentConfig`.
- **MCP:** usa las tools MCP de `eino-ext` y regístralas igual.
- **RAG:** un retriever de Eino como tool, o en el armado de mensajes de `Reply`.

## Tests

```bash
cd backend && go vet ./... && go test ./...
cd frontend && npm run check && npm run build
```
