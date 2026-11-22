---
name: krat
description: A hands-free voice and vision call with a local agent, drawn as sand settling on a vibrating steel plate.
colors:
  steel: "#1e2a33"
  steel-plate: "#27343e"
  steel-line: "#3a4955"
  sand: "#eae3d2"
  sand-dim: "#a9a597"
  brass: "#c9a44c"
  coral: "#e46f5a"
typography:
  display:
    fontFamily: "Atkinson Hyperlegible Next, system-ui, sans-serif"
    fontSize: "clamp(1.5rem, 2.6vw, 2.125rem)"
    fontWeight: 600
    lineHeight: 1.45
    letterSpacing: "-0.015em"
  headline:
    fontFamily: "Atkinson Hyperlegible Next, system-ui, sans-serif"
    fontSize: "clamp(1.25rem, 2.2vw, 1.875rem)"
    fontWeight: 400
    lineHeight: 1.35
  title:
    fontFamily: "Atkinson Hyperlegible Next, system-ui, sans-serif"
    fontSize: "1.625rem"
    fontWeight: 600
    lineHeight: 1.45
    letterSpacing: "-0.01em"
  body:
    fontFamily: "Atkinson Hyperlegible Next, system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: "Atkinson Hyperlegible Next, system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 400
    lineHeight: 1.45
    fontFeature: "tnum"
rounded:
  pill: "999px"
  plate: "10px"
  frame: "8px"
spacing:
  xs: "8px"
  tight: "10px"
  sm: "12px"
  md: "16px"
  lg: "24px"
  xl: "28px"
  gutter: "40px"
components:
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.sand}"
    typography: "{typography.body}"
    rounded: "{rounded.pill}"
    padding: "0 20px"
    height: "48px"
  button-ghost-hover:
    backgroundColor: "{colors.steel-plate}"
  button-toggled-off:
    backgroundColor: "{colors.sand}"
    textColor: "{colors.steel}"
    rounded: "{rounded.pill}"
    padding: "0 20px"
    height: "48px"
  button-start:
    backgroundColor: "{colors.sand}"
    textColor: "{colors.steel}"
    rounded: "{rounded.pill}"
    padding: "0 28px"
    height: "48px"
  button-hangup:
    backgroundColor: "{colors.coral}"
    textColor: "{colors.steel}"
    rounded: "{rounded.pill}"
    padding: "0 20px"
    height: "48px"
  select-voice:
    backgroundColor: "transparent"
    textColor: "{colors.sand}"
    rounded: "{rounded.pill}"
    padding: "0 44px 0 20px"
    height: "48px"
  status-chip:
    backgroundColor: "transparent"
    textColor: "{colors.sand}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "8px 16px"
  plate:
    backgroundColor: "{colors.steel-plate}"
    rounded: "{rounded.plate}"
    width: "min(520px, calc(100vw - 32px), calc(100dvh - 330px))"
  self-view:
    backgroundColor: "{colors.steel-plate}"
    rounded: "{rounded.frame}"
    width: "300px"
---

# Design System: krat

## Overview

**Creative North Star: "Sand on Blued Steel"**

krat has no face. Its voice is shown as a Chladni figure: grains of sand on a square steel plate gather on the still nodal lines of whatever mode the plate is driven at. Each call state gets its own figure. While krat thinks or speaks, the figures cycle and the grains move with the live audio. Everything else on the screen is cut from the same material: a blued-steel ground, sand-colored type and particles, hairline steel edges. The dark ground comes from the material itself, a dark plate under light sand. It was not chosen because "call apps are dark".

There is one screen, and it is calm and sparse. The plate is the protagonist, centered and up to 520px square. The status chip, captions and pill controls stay quiet so the plate and the conversation carry the screen. Colour is rationed by meaning. Sand is the default ink. Brass appears only when krat is listening to you or showing your words. Coral appears only on hang up and on errors. The system deliberately refuses the category default, a glowing gradient orb over round icon buttons: no glow, no gradients and no drop shadows anywhere.

Type is set in Atkinson Hyperlegible Next at two weights, sized so captions can be read from arm's length while the person talks. krat's reply is the largest live text on screen. Your own words sit above it, smaller and dimmer.

**Key Characteristics:**
- A square steel plate with a sand particle field is the single signature element and the agent's only visual identity.
- The ground is blued steel and sand is the ink. Every colour comes from that physical palette.
- Accent colours carry meaning: brass means "you / listening", coral means "end / error". Neither is used for decoration.
- Depth is a single 1px inset steel hairline. Nothing casts a shadow.
- Controls are hairline steel pills. Only primary actions are filled.

## Colors

A low-chroma, cool blued-steel ground with warm sand ink, plus two strictly rationed signal colours.

### Primary
- **Sand** (`sand`): the ink of the whole system. All primary text, krat's reply captions, the plate's particles in every state except "hearing", the filled Start-call pill, the inverted "toggled off" control state, focus rings, text selection, caret and native `accent-color`. Contrast is 11.45:1 on steel.

### Secondary
- **Brass** (`brass`): reserved for "listening / you". It colours the status dot in the listening and hearing states, the "Tú:" prefix on your transcribed line, and the plate's particles while you are speaking (hearing). Contrast is 6.2:1 on steel.

### Tertiary
- **Signal Coral** (`coral`): reserved for ending and failing. It fills the Hang up pill (steel text, 4.69:1), colours inline error notices, and colours the status dot when the connection is offline.

### Neutral
- **Blued Steel** (`steel`): the page ground, `theme-color`, and the background of native `<option>` lists. Steel is also the text colour on every sand or coral fill.
- **Steel Plate** (`steel-plate`): the plate's surface and the self-view frame's surface, and the hover fill for ghost pills and the voice select. Sitting slightly lighter than the ground is what makes the plate read as an object.
- **Steel Line** (`steel-line`): every hairline. Used for the 1px borders of pills, the chip and the select, and for the inset edge of the plate and self-view. It sits at 1.58:1 on steel, so it outlines shapes without becoming a graphic element.
- **Dim Sand** (`sand-dim`): secondary text. Used for the model name beside the wordmark, hints, your transcribed line, an interrupted reply and its "· interrumpida" mark, camera-off captions, the select's chevron, and the idle status dot. Contrast is 5.94:1 on steel and 5.17:1 on plate.

### Named Rules
**The Brass Is You Rule.** Brass appears only when krat is listening to you or is quoting you: the listening/hearing dot, the "Tú:" prefix, and the hearing-state particles. Never use it for focus, selection, links, hover or decoration. Focus rings, selection, caret and `accent-color` are all sand.

**The Coral Ends Things Rule.** Coral is used only for hang up, error notices and the offline dot. A coral element on screen means "this stops the call" or "something went wrong", nothing else.

**The Material Palette Rule.** Every colour is a material from the plate world: steel, sand, brass fittings, and a warning paint. Do not introduce a colour that has no place in that world.

## Typography

**Display Font:** Atkinson Hyperlegible Next (with system-ui, sans-serif). It is self-hosted through `@fontsource`, with only the 400 and 600 weights loaded.

**Character:** a hyperlegible grotesque whose letterforms stay distinct at a glance. It suits captions read mid-conversation. The two weights do all the work: 600 marks names and calls to action, and 400 is used for everything spoken.

### Hierarchy
- **Display** (600, clamp(1.5rem, 2.6vw, 2.125rem), tracking -0.015em, balanced wrap): the single heading "Habla con krat", shown only when no call is active.
- **Headline** (400, clamp(1.25rem, 2.2vw, 1.875rem), line-height 1.35, pretty wrap): krat's reply caption. During a call it is the largest live text on screen.
- **Title** (600, 1.625rem, tracking -0.01em; 1.375rem at 760px and below): the lowercase "krat" wordmark. The model name beside it is set at 400, 0.9375rem, in dim sand.
- **Body** (400, 1.0625rem, line-height 1.45): hints and explanatory copy in dim sand. Your transcribed line uses the same face at 1.125rem in dim sand. Controls and inline notices are set at 1rem.
- **Label** (400, 0.9375rem, tabular figures; 0.8125rem on narrow screens): the status chip. Tabular figures keep the elapsed seconds ("Pensando · 3,2 s") from jittering. Camera captions are set at 0.875rem.

### Named Rules
**The Two Weights Rule.** Only 400 and 600 are loaded, so only 400 and 600 may be used. 600 is reserved for the wordmark, the idle heading and the filled Start and Hang up pills.

**The Reply Outranks You Rule.** krat's reply is always larger and brighter than your transcribed line. You read what krat said, and only glance at what you said.

## Layout

The screen is a three-row grid (header, main, footer) at `min-height: 100dvh`. The header is a flex strip with the wordmark on the left and the status chip on the right. The main row is a centered column: the plate, then the captions block (max 62ch, centered text, min-height 7.5rem so the controls don't jump as captions come and go). The footer holds a centered, wrapping row of controls.

The plate's size is a single custom property, `--plate-size: min(520px, calc(100vw - 32px), calc(100dvh - 330px))`, so the plate shrinks to keep the whole call in one viewport. On desktop the self-view (300px wide, 3:2) is absolutely positioned at the top right of the main column, beside the plate and not on it.

Spacing rhythm: 40px page gutters, a 28px gap between plate and captions, 12px between controls, and 10px inside the chip and between caption lines. Header padding is 24px 40px and footer padding is 24px 40px 32px.

At 760px and below, gutters drop to 16px and the plate-to-caption gap to 20px. The plate is sized with `calc(100dvh - 380px)`. The self-view shrinks to 112px and overlaps the plate's top-right corner. Controls no longer wrap. The model and voice selects grow to fill the row. The mute, camera and hang-up pills become icon-only: their labels stay in the DOM as screen-reader text. With no call active, the footer stacks into a column, with the full-width select above a full-width Start-call pill.

## Elevation & Depth

The system is flat and has exactly one depth device: a 1px inset steel hairline (`box-shadow: inset 0 0 0 1px` in steel line) on a surface one step lighter than the ground. The plate and the self-view frame use it. Pills and the chip use an ordinary 1px steel-line border. Nothing casts a shadow and nothing glows.

### Shadow Vocabulary
- **Inset hairline** (`box-shadow: inset 0 0 0 1px var(--steel-line)`): the edge of the plate and of the self-view frame. Used only on those two. This is the only shadow value in the system.

### Named Rules
**The One Hairline Rule.** Depth comes only from a lighter steel surface plus the 1px inset hairline. Never use drop shadows, glows, blur or gradients for depth.

## Shapes

There are two shape families. Rectangles with small radii are things you look at: the plate at 10px, the self-view at 8px. Full pills (999px) are things you touch or read status from: every button, the voice select and the status chip. The status dot is an 8px circle. The plate is always square (`aspect-ratio: 1`), and its inner padding is clamp(10px, 2.4%, 16px) so the sand never touches the edge. The self-view is always 3:2 and is mirrored like any self-view.

## Components

### Buttons
Quiet hairline pills. Only the action that starts or ends the call is filled.
- **Shape:** full pill (999px), 48px tall, 1px steel-line border, a 20px Lucide icon with a 10px gap before the label.
- **Ghost (mute, camera):** transparent with sand text and 20px side padding. Hover fills with steel plate over 120ms ease-out.
- **Toggled off (mic muted, camera off):** inverts to a sand fill and border with steel text, and exposes `aria-pressed="true"`. The control that is "off" is the one that stands out.
- **Start call:** sand fill, steel text, weight 600, 28px side padding. Shown only when no call is active. It keeps its label at every width.
- **Hang up:** coral fill, steel text, weight 600. Shown only during a call.
- **Hover on filled pills:** `filter: brightness(1.06)`.
- **Focus:** a 2px sand outline at 3px offset, global for every focusable element.

### Status Chip
- **Style:** a transparent pill with a 1px steel-line border, 8px 16px padding, label type with tabular figures, and `role="status"` with `aria-live="polite"`.
- **Dot:** an 8px circle before the text. It is dim sand when idle or busy, brass when listening or hearing, sand when speaking, and coral when offline.
- **Copy:** each state names what krat is doing, with elapsed seconds while waking or thinking and the response latency while speaking ("Hablando · respondió en 2,4 s").

### Inputs / Fields
- **Voice select:** a native `<select>` with `appearance: none`, styled as a ghost pill (48px tall, 0 44px 0 20px padding). A 16px dim-sand chevron sits 18px from the right edge. Hover fills with steel plate. Options render steel on sand. Narrow screens use short labels ("Femenina · ES") because native selects ignore `text-overflow`.

### Captions
- **Idle:** the display heading plus a dim-sand hint.
- **In call:** your line is in dim sand at 1.125rem with a brass "Tú:" prefix, and shows "…" at 60% opacity while the transcript is pending. krat's reply is headline size in sand. An interrupted reply turns dim sand and gets a small "· interrumpida" mark. Errors appear below as coral body text with `role="alert"`.

### Self-view
- **Style:** a steel-plate frame with an 8px radius and the inset hairline, 300px wide at 3:2 (112px wide on narrow screens). Hidden when no call is active. With the camera off or denied, it shows a centered VideoOff icon and a dim-sand caption ("Cámara apagada", "Sin cámara: krat solo te oye.").

### Chladni Plate (signature)
A square canvas of 3,200 sand grains on the steel plate. Each grain is pushed in proportion to how much its spot vibrates under f = cos(nπx)cos(mπy) ± cos(mπx)cos(nπy), so the grains drift onto the nodal lines. Each call state has its own (n, m, sign) mode and a base drive:
- **Off and starting** (1,3,−). **Waking and offline** (1,2,−), driven gently.
- **Muted** (1,3,+). **Listening** (2,4,+). **Hearing** (2,5,+), with grains drawn in brass.
- **Thinking** cycles four modes every 1600ms. **Speaking** cycles four modes every 3000ms.
- The live audio level (yours, or krat's voice while it speaks) is added to the drive, so the sand moves with the sound.

Grains are 1.8px squares at 55% alpha, so lines where many grains pile up read brighter than stray grains. The canvas is `aria-hidden`, because the status chip carries the meaning. Under `prefers-reduced-motion` there is no continuous motion: each new figure settles at once and is held, and the mode cycles stop.

## Do's and Don'ts

### Do:
- **Do** let the plate be the only representation of krat. Express new agent states as new plate modes and drive levels.
- **Do** use sand for every focus ring, selection, caret and `accent-color` (2px outline, 3px offset).
- **Do** keep brass to "listening / you" and coral to hang up, errors and offline.
- **Do** build new controls as 48px hairline pills in steel line, filled only when they start or end something or show a toggled-off state.
- **Do** use the 1px inset hairline on a steel-plate surface for anything that must read as an object on the ground.
- **Do** show elapsed time or latency in the status chip with tabular figures whenever krat is busy.
- **Do** keep icon-only controls' labels in the DOM as screen-reader text.

### Don't:
- **Don't** use drop shadows, glows, gradients or blur anywhere.
- **Don't** replace or decorate the plate with an orb, an avatar or a face.
- **Don't** use brass for focus, links, hover or decoration, or coral for anything that isn't ending or failing.
- **Don't** load or use any weight other than 400 and 600.
- **Don't** add a light theme by inverting tokens. The dark ground comes from the steel plate and its sand, not from a theme toggle.
- **Don't** introduce colours outside the steel, sand, brass and coral palette.
