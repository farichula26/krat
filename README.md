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

Todo pasa por `newAgent` en `backend/agent.go`:
- **Tools:** añade tools de Eino en `ToolsConfig` del `ChatModelAgentConfig`.
- **MCP:** usa las tools MCP de `eino-ext` y regístralas igual.
- **RAG:** un retriever de Eino como tool, o en el armado de mensajes de `Reply`.

## Tests

```bash
cd backend && go vet ./... && go test ./...
cd frontend && npm run check && npm run build
```
