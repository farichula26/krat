# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack
Backend Go (`backend/`, `go run .`); frontend Svelte 5 + Vite + TypeScript (`frontend/`, `npm run dev`). Chosen by the user.

## Users
Farid and his team. They use it as an internal lab: sitting at a desk with a webcam and microphone, they talk hands-free to a multimodal agent, show it objects, and judge how well it sees, hears and answers. Later they will add tools, MCP and RAG to the same agent.

## Product Purpose
A real-time voice-and-vision conversation with **krat**, an agent that runs on a live speech-to-speech model chosen in the backend's `.env`. The page never names the model or its provider. It plays the same role Anam.ai does. Success means the conversation feels like a call: you speak, krat answers aloud in a few seconds, you can interrupt it, and it can describe what you hold up to the camera.

## Positioning
Inspectable: the backend, prompt, tools and transport are owned by the team, so new tools and knowledge can be wired in without depending on a hosted avatar service.

## Operating Context
- Desktop browser, webcam and mic, headphones or speakers with browser echo cancellation.
- The model is reached through the backend; there is no cold start and answers begin in about a second.
- Spanish is the main language; the model answers in the language it is spoken to.

## Capabilities and Constraints
- Hands-free turns with voice activity detection, and barge-in (talking while krat speaks stops it; the model's server-side detection decides).
- One webcam frame per second is streamed during the call.
- Captions show what the user said (from model transcription) and what krat answers.
- Voice selectable among the model's 30 voices before the call; default `Kore`.
- There is no generated video avatar: krat is represented by an abstract animated form.
- Undecided: authentication, conversation persistence, multi-user, deployment.

## Brand Commitments
- The agent's name is **krat** (lowercase, as the user wrote it).

## Evidence on Hand
No logo, brand assets, or real usage data yet. Do not invent metrics or claims about the model.

## Product Principles
1. The conversation comes first: voice and camera are the interface; the screen only supports them.
2. Always show what krat is doing (waking, listening, thinking, speaking) so latency never reads as a failure.
3. Be honest about a small local model: show its words as they arrive and make errors and reconnections plain.
4. Leave room to grow: tools and knowledge will join later without changing how a call looks.
