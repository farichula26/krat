# Agente omni por voz + webcam (estilo Anam.ai) — Go/Eino + Svelte

## Contexto
Farid quiere una web donde ve su webcam, habla con un agente por voz manos libres, le muestra objetos y el agente responde hablando. Es **base de un producto**: backend Go con Eino (`backend/`, `go run .`) preparado para tools/MCP/RAG; frontend Svelte (`frontend/`, `npm run dev`). Repo vacío (solo `backend/`, `frontend/` vacías, sin commits).

Modelo: `lfm3-omni:latest` (1.7B, Q4) en Ollama 0.40.17 vía túnel `https://burton-tax-communications-typical.trycloudflare.com` (cambia al reiniciar el túnel → va en `.env`).

### Hechos verificados contra el servidor
- Capacidades: `completion, vision, audio, speech, tools`. Voces incl. `female-spanish`, `male-spanish` (lista en `/api/show` → `voices`).
- **Audio de entrada**: WAV en base64 dentro de `images` de `/api/chat` → funciona (1,6 s). Sin system prompt el modelo **transcribe** en vez de responder; con prompt "responde, no transcribas" contesta bien.
- **Voz de salida**: `POST /v1/audio/speech {model,input,voice}` → WAV 16-bit mono 24 kHz, streaming. Caliente: 0,8 s al primer byte; en frío: 28 s (carga del modelo).
- No hay entrada de vídeo nativa → se mandan fotogramas JPEG como imágenes.

## Decisiones (tomadas con Farid)
- Avatar: **orbe abstracto animado** (respira en reposo, se tensa al escuchar, vibra con la amplitud de su voz).
- Turnos: **manos libres + interrupción** (barge-in).
- Voz: **selector en la UI**, por defecto `female-spanish`.
- Transporte: **A. WebSocket por turnos** (descartados: WebRTC/Pion — 3× código sin ganar latencia porque el modelo procesa clips completos; HTTP+SSE — interrupción torpe).

## Decisiones mías (corregibles)
- VAD en el navegador con `@ricky0123/vad-web` (Silero). `getUserMedia` con `echoCancellation` para que el agente no se escuche a sí mismo.
- Por turno se envía: WAV 16 kHz del habla + 1 fotograma JPEG (≈640 px) capturado al terminar de hablar.
- Historial: solo el turno actual lleva audio+imagen; los anteriores se guardan como texto. El texto del usuario sale de una **llamada de transcripción en paralelo** (mismo modelo, prompt "transcribe literalmente") → sirve para subtítulos e historial sin añadir latencia. Últimos 20 turnos.
- Eino ADK `ChatModelAgent` sin tools por ahora: añadir tools/MCP luego = registrarlas.
- Al arrancar, el backend precalienta el modelo (llamada mínima de chat + TTS) para evitar los 28 s del primer turno; la UI muestra "Despertando…" hasta que el backend avisa `ready`.
- Config por env con `.env` opcional (cargador mínimo propio, sin dependencia): `OLLAMA_URL`, `OLLAMA_MODEL=lfm3-omni:latest`, `PORT=8080`.

## Arquitectura

```
Navegador (Svelte)                         Backend Go (Eino)                      Ollama
 webcam <video> ─ frame JPEG ─┐
 mic ─ Silero VAD ─ WAV ──────┼─ WS {turn} ─▶ session ──┬─ transcribe (paralelo) ─▶ /api/chat
                              │                         └─ ADK agent.Run (stream) ─▶ /api/chat
 cola de audio ◀─ {audio} ────┼─◀ por frase ── splitter ─▶ tts ───────────────────▶ /v1/audio/speech
 orbe ◀─ AnalyserNode         │
 habla durante respuesta ─ WS {interrupt} ─▶ cancel(ctx)
```

### Protocolo WS (JSON, audio en base64 — en localhost el 33 % extra no importa y evita desincronizar frames binarios)
Cliente → servidor
- `{"type":"turn","id":n,"audio":"<wav b64>","image":"<jpeg b64>","voice":"female-spanish"}`
- `{"type":"interrupt"}`

Servidor → cliente (todo lleva `id` del turno; el cliente descarta ids viejos tras interrumpir)
- `{"type":"ready"}` · `{"type":"transcript","id","text"}` · `{"type":"delta","id","text"}`
- `{"type":"audio","id","seq","data":"<wav b64>"}` · `{"type":"done","id"}` · `{"type":"error","message"}`

HTTP: `GET /api/voices` (de `/api/show`), `GET /healthz`.

## Ficheros

### backend/ (módulo `agente/backend`)
- `main.go` — carga `.env`, config, mux (`/ws`, `/api/voices`, `/healthz`), precalentamiento.
- `agent.go` — `ollama.NewChatModel` (eino-ext) + `adk.NewChatModelAgent` con instrucción en español (breve, conversacional, 1-3 frases, describe lo que ve si es relevante, nunca transcribe). Función que arma el mensaje multimodal (`UserInputMultiContent`: texto + imagen JPEG + audio WAV) y devuelve el stream.
- `session.go` — bucle WS por conexión (`github.com/coder/websocket`): recibe turnos, cancela el turno en curso con `context.CancelFunc` al llegar `interrupt` o un turno nuevo, mantiene historial, lanza transcripción en paralelo, alimenta splitter → TTS en orden (`seq`).
- `ollama.go` — HTTP directo para lo que Eino no cubre: `speech(ctx,text,voice)`, `voices(ctx)`, transcripción si hace falta.
- `sentence.go` + `sentence_test.go` — corta el stream de texto en frases (`.!?…\n` + espacio, mínimo ~20 caracteres, vacía el resto al final). Test de tabla.
- `.env.example`, `.gitignore` (`.env`).

**Riesgo a verificar primero (spike de 10 min):** que el adaptador `eino-ext/components/model/ollama` pase la parte de audio a `images`. Plan B: enviarla como parte `Image` con `MIMEType: audio/wav` (Ollama detecta el formato por contenido). Plan C: implementar `model.ToolCallingChatModel` propio sobre el cliente `github.com/ollama/ollama/api` (sigue siendo Eino, el agente no cambia). Comprobar también la versión de Go que pide Eino (local: 1.23.6; `GOTOOLCHAIN=auto` la descarga).

### frontend/ (Vite + Svelte 5 + TS)
- `vite.config.ts` — proxy `/ws` y `/api` → `localhost:8080` (un solo origen, sin CORS).
- `src/App.svelte` — pantalla de llamada.
- `src/lib/session.svelte.ts` — estado ($state): `waking | idle | listening | thinking | speaking | error`, WS con reconexión, turnos, interrupción.
- `src/lib/audio.ts` — codificar Float32→WAV 16 kHz; cola de reproducción Web Audio con `AnalyserNode`; `stop()` para barge-in.
- `src/lib/camera.ts` — `getUserMedia`, captura de fotograma a JPEG vía canvas.
- `src/lib/Orb.svelte` — orbe (canvas/CSS) dirigido por estado + amplitud.
- Assets de VAD (onnx/wasm) servidos localmente desde `node_modules` (vite-plugin-static-copy o `baseAssetPath`), sin depender de CDN.

### UI (se refina con /impeccable y /frontend-design al implementar)
```
┌──────────────────────────────────────────────────┐
│  ● Conectado                     Voz: [Femenina▾]│
│                                                  │
│            ╭───────────╮                         │
│           (    orbe     )   ← protagonista       │
│            ╰───────────╯                         │
│   "Veo una taza roja. ¿Es tuya?"   ← subtítulo   │
│   tú: "¿qué tengo en la mano?"                   │
│                               ┌──────────┐       │
│                               │  webcam  │ PiP   │
│   [🎙 silenciar] [📷 cámara] [✕ colgar]└──────────┘│
└──────────────────────────────────────────────────┘
```
Estados accesibles (texto + `aria-live` para subtítulos), `prefers-reduced-motion` calma el orbe, layout móvil apilado. Permiso de cámara denegado → sigue solo con voz y lo indica.

## Orden de trabajo
0. Commit del diseño en `docs/superpowers/specs/2026-11-22-agente-omni-design.md` (copia de este documento).
1. Spike Eino+audio (riesgo de arriba) → decide plan A/B/C.
2. Backend: config, agent, ollama (tts/voices), splitter+test, session WS, precalentamiento.
3. Frontend: scaffold Vite Svelte-TS, audio/camera/session, orbe y pantalla (cargar skills /impeccable y /frontend-design aquí).
4. Verificación end-to-end con /agent-browser.

## Verificación
- `cd backend && go vet ./... && go test ./...` (splitter).
- `cd backend && go run .` → `curl localhost:8080/healthz`, `curl localhost:8080/api/voices`.
- Prueba WS con un WAV real (el de TTS de "¿Cuál es la capital de Francia?") → llegan `transcript`, `delta`, `audio`, `done`.
- `cd frontend && npm run check && npm run build`.
- `npm run dev` + **agent-browser** con Chrome falso de medios (`--use-fake-ui-for-media-stream --use-fake-device-for-media-stream --use-file-for-fake-audio-capture=<wav>`): comprobar que pasa por listening → thinking → speaking, aparecen subtítulos, no hay errores de consola; capturas a 1280 px y 390 px.
- Prueba manual de Farid con webcam/mic reales: mostrar un objeto, interrumpir a mitad de respuesta.

## Fuera de alcance (por ahora)
Tools/MCP/RAG reales, auth, persistencia de conversaciones, avatar de vídeo, despliegue, multiusuario.

## Cambios durante la implementación (2026-11-22)
- **Audio de entrada vía Eino:** el adaptador `eino-ext/components/model/ollama` v0.1.9 rechaza las partes de audio. El WAV va como parte `Image` con `MIMEType: audio/wav` (plan B) y Ollama lo detecta como audio.
- **Transcripción:** los prompts en español hacen que el modelo responda en vez de transcribir. Se usa el system prompt `Perform ASR.` y la llamada se lanza **después** de generar la respuesta: en paralelo retrasaba a krat ~1 s, porque Ollama comparte el modelo.
- **Voz:** se pide `response_format: pcm` y se reenvía en trozos de 0,2 s mientras Ollama la genera. El primer audio bajó de ~5 s a ~2,5–3,4 s.
- **Toolchain:** `go.mod` fija `toolchain go1.25.5`, porque `bytedance/sonic` (dependencia de Eino) no enlaza con Go 1.24.0.
- **Configuración:** `.env` tiene prioridad sobre el entorno del shell, porque el shell del usuario exporta otro `OLLAMA_URL`.
- **Agente e interfaz:** el agente se llama krat. El mundo visual es la «Placa de Chladni», elegida en la ronda de /impeccable.
