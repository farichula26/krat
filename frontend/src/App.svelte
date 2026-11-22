<script lang="ts">
  import { onMount } from 'svelte'
  import { ChevronDown, Mic, MicOff, Phone, PhoneOff, Video, VideoOff } from '@lucide/svelte'
  import Plate from './lib/Plate.svelte'
  import { Call } from './lib/call.svelte'

  const call = new Call()
  const narrow = matchMedia('(max-width: 760px)')
  let short = $state(narrow.matches)
  narrow.addEventListener('change', e => (short = e.matches))
  let now = $state(performance.now())
  let videoEl = $state<HTMLVideoElement>()

  onMount(() => {
    call.loadModels()
    const t = setInterval(() => (now = performance.now()), 100)
    return () => clearInterval(t)
  })

  $effect(() => {
    if (videoEl) videoEl.srcObject = call.video
    call.attachVideo(videoEl ?? null)
  })

  const secs = (s: number) => `${s.toLocaleString('es', { minimumFractionDigits: 1, maximumFractionDigits: 1 })} s`
  const elapsed = $derived(secs(Math.max(0, now - call.since) / 1000))

  const statusText = $derived.by(() => {
    switch (call.status) {
      case 'off': return 'Fuera de llamada'
      case 'starting': return 'Preparando micrófono y cámara'
      case 'waking': return `Despertando a krat · ${elapsed}`
      case 'listening': return 'Escuchando'
      case 'hearing': return 'Hablas tú'
      case 'thinking': return `Pensando · ${elapsed}`
      case 'speaking': return call.latency ? `Hablando · respondió en ${secs(call.latency)}` : 'Hablando'
      case 'muted': return 'Micrófono silenciado'
      case 'offline': return 'Sin conexión con el servidor · reintentando'
    }
  })

  const LANG: Record<string, string> = {
    'american-english': 'inglés (EE. UU.)', 'british-english': 'inglés (R. U.)', arabic: 'árabe', chinese: 'chino',
    danish: 'danés', dutch: 'neerlandés', finnish: 'finés', french: 'francés', german: 'alemán', greek: 'griego',
    hebrew: 'hebreo', italian: 'italiano', japanese: 'japonés', korean: 'coreano', polish: 'polaco',
    portuguese: 'portugués', spanish: 'español', swedish: 'sueco', turkish: 'turco',
  }
  const CODE: Record<string, string> = {
    'american-english': 'EN-US', 'british-english': 'EN-GB', arabic: 'AR', chinese: 'ZH', danish: 'DA', dutch: 'NL',
    finnish: 'FI', french: 'FR', german: 'DE', greek: 'EL', hebrew: 'HE', italian: 'IT', japanese: 'JA', korean: 'KO',
    polish: 'PL', portuguese: 'PT', spanish: 'ES', swedish: 'SV', turkish: 'TR',
  }
  // Native selects ignore text-overflow, so narrow screens get short labels ("Femenina · ES").
  function voiceName(v: string, short: boolean) {
    if (v === 'default') return 'Predeterminada'
    if (!v.includes('-')) return v // Gemini voices are plain names ("Kore")
    const [sex, ...rest] = v.split('-')
    const lang = rest.join('-')
    const l = short ? (CODE[lang] ?? lang) : (LANG[lang] ?? rest.join(' '))
    return `${sex === 'female' ? 'Femenina' : sex === 'male' ? 'Masculina' : sex} · ${l}`
  }
</script>

<div class="app" data-status={call.status}>
  <header>
    <p class="name">krat <span>{call.model.label}</span></p>
    <p class="chip" role="status" aria-live="polite">
      <span class="dot" aria-hidden="true"></span>{statusText}
    </p>
  </header>

  <main>
    <Plate status={call.status} level={call.level} />

    <figure class="cam" class:hidden={!call.live}>
      <!-- svelte-ignore a11y_media_has_caption -->
      <video bind:this={videoEl} autoplay playsinline muted class:on={call.camOn}></video>
      {#if !call.camOn}
        <figcaption><VideoOff size={20} aria-hidden="true" />{call.camNote || 'Cámara apagada'}</figcaption>
      {/if}
    </figure>

    <section class="captions" aria-live="polite">
      {#if !call.live}
        <h1>Habla con krat</h1>
        <p class="hint">Te escucha sin pulsar nada, ve lo que le enseñes a la cámara y puedes interrumpirle hablando.</p>
      {:else if call.status === 'waking'}
        <p class="hint">{call.model.provider === 'gemini' ? 'Conectando con Gemini…' : 'Cargando el modelo. La primera vez puede tardar medio minuto.'}</p>
      {:else}
        {#if call.you || call.youPending}<p class="you"><span>Tú:</span> {call.youPending ? '…' : call.you}</p>{/if}
        {#if call.reply}
          <p class="reply" class:cut={call.cut}>{call.reply}{#if call.cut}<span class="cutmark"> · interrumpida</span>{/if}</p>
        {:else if call.status === 'listening'}
          <p class="hint">Di algo, o enséñale algo a la cámara y pregúntale qué ve.</p>
        {/if}
      {/if}
      {#if call.notice}<p class="notice" role="alert">{call.notice}</p>{/if}
    </section>
  </main>

  <footer>
    {#if call.live}
      <button class="ctl" class:off={!call.micOn} aria-pressed={!call.micOn} onclick={() => call.toggleMic()}>
        {#if call.micOn}<Mic size={20} aria-hidden="true" />{:else}<MicOff size={20} aria-hidden="true" />{/if}
        <span>{call.micOn ? 'Silenciar' : 'Activar micro'}</span>
      </button>
      <button class="ctl" class:off={!call.camOn} aria-pressed={!call.camOn} onclick={() => call.setCamera(!call.camOn)}>
        {#if call.camOn}<Video size={20} aria-hidden="true" />{:else}<VideoOff size={20} aria-hidden="true" />{/if}
        <span>{call.camOn ? 'Apagar cámara' : 'Encender cámara'}</span>
      </button>
    {/if}
    {#if !call.live && call.models.length > 1}
      <label class="voice">
        <span class="sr">Modelo</span>
        <select value={call.modelId} onchange={e => call.setModel(e.currentTarget.value)}>
          {#each call.models as m (m.id)}
            <option value={m.id}>{m.label}</option>
          {/each}
        </select>
        <ChevronDown size={16} aria-hidden="true" />
      </label>
    {/if}
    <label class="voice">
      <span class="sr">Voz de krat</span>
      <select value={call.voice} disabled={call.voiceLocked} title={call.voiceLocked ? 'Con Gemini la voz se elige antes de llamar' : undefined} onchange={e => call.setVoice(e.currentTarget.value)}>
        {#each call.voices as v (v)}
          <option value={v}>{voiceName(v, short)}</option>
        {/each}
      </select>
      <ChevronDown size={16} aria-hidden="true" />
    </label>
    {#if call.live}
      <button class="ctl end" onclick={() => call.hangup()}><PhoneOff size={20} aria-hidden="true" /><span>Colgar</span></button>
    {:else}
      <button class="ctl start" onclick={() => call.start()}><Phone size={20} aria-hidden="true" /><span>Empezar llamada</span></button>
    {/if}
  </footer>
</div>

<style>
  .app {
    min-height: 100dvh;
    display: grid;
    grid-template-rows: auto 1fr auto;
    --plate-size: min(520px, calc(100vw - 32px), calc(100dvh - 330px));
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 24px 40px;
  }
  .name {
    font-size: 1.625rem;
    font-weight: 600;
    letter-spacing: -0.01em;
  }
  .name span {
    margin-left: 10px;
    font-size: 0.9375rem;
    font-weight: 400;
    color: var(--sand-dim);
  }
  .chip {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 16px;
    border: 1px solid var(--steel-line);
    border-radius: 999px;
    font-size: 0.9375rem;
    font-variant-numeric: tabular-nums;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--sand-dim);
  }
  [data-status='listening'] .dot, [data-status='hearing'] .dot { background: var(--brass); }
  [data-status='speaking'] .dot { background: var(--sand); }
  [data-status='offline'] .dot { background: var(--coral); }

  main {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 28px;
    padding: 0 40px;
  }
  .cam {
    position: absolute;
    top: 0;
    right: 40px;
    width: 300px;
    aspect-ratio: 3 / 2;
    margin: 0;
    border-radius: 8px;
    overflow: hidden;
    background: var(--steel-plate);
    box-shadow: inset 0 0 0 1px var(--steel-line);
  }
  .cam.hidden { display: none; }
  video {
    width: 100%;
    height: 100%;
    object-fit: cover;
    transform: scaleX(-1); /* mirror, like every self-view */
    visibility: hidden;
  }
  video.on { visibility: visible; }
  figcaption {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 12px;
    text-align: center;
    font-size: 0.875rem;
    color: var(--sand-dim);
  }

  .captions {
    width: min(100%, 62ch);
    min-height: 7.5rem;
    text-align: center;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  h1 {
    font-size: clamp(1.5rem, 2.6vw, 2.125rem);
    font-weight: 600;
    letter-spacing: -0.015em;
    text-wrap: balance;
  }
  .hint { color: var(--sand-dim); font-size: 1.0625rem; text-wrap: pretty; }
  .you { color: var(--sand-dim); font-size: 1.125rem; }
  .you span { color: var(--brass); }
  .reply {
    font-size: clamp(1.25rem, 2.2vw, 1.875rem);
    line-height: 1.35;
    text-wrap: pretty;
  }
  .reply.cut { color: var(--sand-dim); }
  .cutmark { font-size: 0.6em; color: var(--sand-dim); }
  .notice { color: var(--coral); font-size: 1rem; }

  footer {
    display: flex;
    justify-content: center;
    flex-wrap: wrap;
    gap: 12px;
    padding: 24px 40px 32px;
  }
  .ctl, select {
    height: 48px;
    border: 1px solid var(--steel-line);
    border-radius: 999px;
    background: transparent;
    color: var(--sand);
    font: inherit;
    font-size: 1rem;
  }
  .ctl {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    padding: 0 20px;
    cursor: pointer;
    transition: background-color 120ms ease-out;
  }
  .ctl:hover { background: var(--steel-plate); }
  .ctl.off { background: var(--sand); border-color: var(--sand); color: var(--steel); }
  .ctl.end { background: var(--coral); border-color: var(--coral); color: var(--steel); font-weight: 600; }
  .ctl.start { background: var(--sand); border-color: var(--sand); color: var(--steel); font-weight: 600; padding: 0 28px; }
  .ctl.end:hover, .ctl.start:hover, .ctl.off:hover { filter: brightness(1.06); }
  .voice { position: relative; display: inline-flex; align-items: center; }
  select {
    appearance: none;
    padding: 0 44px 0 20px;
    cursor: pointer;
  }
  select:hover:not(:disabled) { background: var(--steel-plate); }
  select:disabled { cursor: not-allowed; opacity: 0.6; }
  option { background: var(--steel); color: var(--sand); }
  .voice :global(svg) { position: absolute; right: 18px; pointer-events: none; color: var(--sand-dim); }

  @media (max-width: 760px) {
    .app { --plate-size: min(calc(100vw - 32px), calc(100dvh - 380px)); }
    header { padding: 16px; }
    .name { font-size: 1.375rem; }
    .name { display: flex; flex-direction: column; min-width: 0; line-height: 1.15; }
    .name span { margin-left: 0; font-size: 0.8125rem; max-width: 17ch; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .chip { font-size: 0.8125rem; padding: 6px 12px; white-space: nowrap; }
    main { padding: 0 16px; gap: 20px; }
    .cam { right: 16px; width: 112px; top: 8px; z-index: 1; }
    figcaption { font-size: 0.6875rem; gap: 4px; padding: 6px; }
    footer { padding: 16px 16px 24px; gap: 8px; flex-wrap: nowrap; }
    [data-status='off'] footer { flex-direction: column; }
    .voice { flex: 1; min-width: 0; }
    select { width: 100%; }
    .ctl:not(.start) span { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
    .ctl:not(.start) { padding: 0 14px; }
  }
</style>
