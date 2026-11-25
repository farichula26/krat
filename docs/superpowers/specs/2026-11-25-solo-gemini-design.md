# Solo Gemini Live, sin nombrarlo en la web

**Intención:** krat usa un único modelo, Gemini 3.8 Live, configurable en `backend/.env` (`GEMINI_MODEL`). La web no menciona Gemini en ningún sitio: ni en los textos ni en la red. Todo lo demás (aspecto, VAD, cámara, carta, prompt, ajustes de Live) se queda igual.

**Decisiones del usuario**
- Se eliminan los demás modelos, incluido el modo local `lfm3-omni`/Ollama/Eino, en backend y frontend.
- "No mencionarlo" abarca el texto visible y la red: `/api/voices` solo devuelve las voces y el WS solo lleva `?voice=`. Los nombres de voz (Kore…) se quedan.

**Enfoques:** un solo camino en vivo (elegido). Se descartó mantener un catálogo o un selector de un único elemento, o esconder el modo local con un flag: deja código muerto y expone el ID del modelo.

**Diseño**
- Backend: `GEMINI_API_KEY` obligatoria (sin ella, el backend no arranca) y `GEMINI_MODEL` con valor por defecto `gemini-3.8-live`. `/api/models` pasa a ser `/api/voices`. `/ws` va siempre a `handleLive`. Los mensajes que ve el usuario dicen "krat", no "Gemini". Se borran `agent.go`, `ollama.go`, `sentence.go` y la parte de turnos de `session.go`.
- Frontend: sin selector de modelo y sin etiqueta junto a "krat". La voz se guarda en `krat.voice`. Se elimina el envío por turnos (`encodeWav`).
- Los docs (README, PRODUCT, DESIGN) pueden nombrar Gemini: van dirigidos al operador, no son la web.

**A criterio del implementador:** los textos exactos de los avisos y los comentarios del código.
