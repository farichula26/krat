---
version: 1
slug: "frontend-src-app-svelte"
primary_target: "frontend/src/App.svelte"
related_targets: []
---

# Surface: call screen (frontend/src/App.svelte)

Mode: operate. The single screen of the app: a live hands-free voice + camera call with krat.

Audience and job: Farid and his team, at a desk with webcam and mic, talk to krat, show it objects, interrupt it, switch voices, and judge how the local model behaves.

Constraints: one screen, no routing. States: waking (model cold start ~30 s), connecting, listening, user speaking, thinking, speaking, interrupted, error/reconnecting, mic denied, camera off/denied. Spanish UI copy.

## Direction contract

THESIS: krat's voice made visible as a Chladni figure: sand on a vibrating steel plate settling into different nodal patterns per state and moving with the actual audio. Refuses the category default of a glowing gradient orb with round buttons.

OWN-WORLD: blued-steel field (#1E2A33) with a lighter plate (#27343E), sand ink (#EAE3D2) for type and particles, brass (#C9A44C) only for "listening / you", coral (#E46F5A) only for hang up and errors. Atkinson Hyperlegible Next, two weights, captions large. Square plate with small radius, pill controls in hairline steel.

STORY: you see that krat is awake and listening, speak, see your words, watch the sand reorganize while it thinks, and hear and read the answer while the sand dances with its voice. Talking over it stops it, and the cut reply stays marked.

FIRST VIEWPORT: header strip (krat name, model, state chip with elapsed seconds); centered square plate ~520px as the protagonist; your webcam 300x200 top-right; captions centered under the plate (you smaller and dimmer, krat large); control bar bottom center: mute, camera, voice select, hang up.

FORM: Chladni plate, position 1 on the grounded list (impeccable's pick). seed:b74709e2dec14c40

SIGNATURE: canvas particle system relaxing toward Chladni nodal lines cos(nπx)cos(mπy) − cos(mπx)cos(nπy); (n,m) per state, jitter driven by live mic/speaker amplitude.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
