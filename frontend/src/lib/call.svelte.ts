import { MicVAD } from '@ricky0123/vad-web'
import { level, Player, toBase64 } from './audio'

export type Status = 'off' | 'starting' | 'waking' | 'listening' | 'hearing' | 'thinking' | 'speaking' | 'muted' | 'offline'

type ServerMsg = { type: string; text?: string; data?: string; message?: string }

const VOICE_KEY = 'krat.voice'
// Higher bar for speech while krat talks, so leftover echo of its own voice doesn't interrupt it.
const THRESHOLD = 0.5
const THRESHOLD_WHILE_SPEAKING = 0.85

// localStorage can throw (private mode, blocked data): the page then simply forgets the choice.
function stored(key: string): string | null {
  try { return localStorage.getItem(key) } catch { return null }
}
function store(key: string, value: string) {
  try { localStorage.setItem(key, value) } catch { /* keep it for this session only */ }
}

// One hands-free call with krat: mic + VAD, camera frames, the WebSocket and playback.
export class Call {
  status = $state<Status>('off')
  since = $state(0) // when the current status began (performance.now)
  you = $state('') // what the user said last
  reply = $state('') // krat's current reply
  cut = $state(false) // the reply was interrupted
  notice = $state('') // problem worth showing (mic denied, model error)
  latency = $state<number | null>(null) // seconds from end of your speech to krat's first sound
  micOn = $state(true)
  camOn = $state(false)
  camNote = $state('')
  video = $state<MediaStream | null>(null)
  voices = $state(['Kore']) // fallback until the server's list arrives
  voice = $state('Kore')

  private ws: WebSocket | null = null
  private vad: MicVAD | null = null
  private player: Player | null = null
  private mic: MediaStream | null = null
  private micAn: AnalyserNode | null = null
  private scratch = new Float32Array(1024)
  private videoEl: HTMLVideoElement | null = null
  private sentAt = 0
  private genDone = false
  private ready = false
  private retry = 0
  private frameTimer: ReturnType<typeof setInterval> | undefined
  private fatal = false // the server refused the call for good (bad key): do not keep retrying

  // Loudness the plate reacts to: krat's voice while it speaks, yours otherwise.
  level = (): number => {
    if (this.status === 'speaking') return this.player?.level() ?? 0
    return this.micAn && this.micOn ? level(this.micAn, this.scratch) : 0
  }

  get live() { return this.status !== 'off' }

  // The voice is fixed when the call connects.
  get voiceLocked() { return this.live }

  attachVideo(el: HTMLVideoElement | null) { this.videoEl = el }

  async loadVoices() {
    let def = this.voice
    try {
      const r = await fetch('/api/voices')
      if (!r.ok) throw new Error(r.statusText)
      const j: { voices: string[]; default: string } = await r.json()
      if (j.voices.length) { this.voices = j.voices; def = j.default }
    } catch { /* keep the fallback: the page still works without the list */ }
    const saved = stored(VOICE_KEY)
    this.voice = saved && this.voices.includes(saved) ? saved : def
  }

  setVoice(v: string) {
    this.voice = v
    store(VOICE_KEY, v)
  }

  async start() {
    this.notice = ''
    this.you = this.reply = ''
    this.fatal = false
    this.set('starting')
    this.player = new Player()
    this.player.onDrain = () => {
      if (this.genDone && this.status === 'speaking') this.toIdle()
    }
    try {
      this.mic = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      })
    } catch {
      this.hangup()
      this.notice = 'krat necesita tu micrófono. Permite el acceso en la barra del navegador y empieza la llamada otra vez.'
      return
    }
    // A track the OS mutes (system mute, hardware switch) delivers pure silence: say so instead of
    // sitting on "Escuchando" forever.
    const track = this.mic.getAudioTracks()[0]
    const MIC_MUTED = 'Tu micrófono está silenciado en el sistema, así que krat no te oye. Actívalo en el control de volumen del escritorio.'
    const onMute = () => { this.notice = MIC_MUTED }
    track.addEventListener('mute', onMute)
    track.addEventListener('unmute', () => { if (this.notice === MIC_MUTED) this.notice = '' })
    if (track.muted) onMute()
    this.micAn = this.player.ctx.createAnalyser()
    this.micAn.fftSize = 1024
    this.player.ctx.createMediaStreamSource(this.mic).connect(this.micAn)
    await this.setCamera(true)

    try {
      this.vad = await MicVAD.new({
        model: 'v5',
        baseAssetPath: '/vad/',
        onnxWASMBasePath: '/vad/',
        getStream: async () => this.mic!,
        pauseStream: async () => {},
        resumeStream: async s => s,
        positiveSpeechThreshold: THRESHOLD,
        negativeSpeechThreshold: 0.35,
        minSpeechMs: 250,
        redemptionMs: 800,
        onSpeechStart: () => this.onSpeechStart(),
        onSpeechEnd: () => this.onSpeechEnd(),
        onVADMisfire: () => { if (this.status === 'hearing') this.toIdle() },
        onFrameProcessed: (_p, frame) => this.feed(frame),
      })
      await this.vad.start()
    } catch (e) {
      console.error('vad', e)
      this.hangup()
      this.notice = 'No se pudo cargar la detección de voz. Recarga la página e inténtalo de nuevo.'
      return
    }
    this.frameTimer = setInterval(() => this.sendFrame(), 1000)
    this.connect()
  }

  hangup() {
    clearInterval(this.frameTimer)
    const ws = this.ws
    this.ws = null
    ws?.close()
    this.vad?.destroy()
    this.vad = null
    this.mic?.getTracks().forEach(t => t.stop())
    this.mic = this.micAn = null
    this.setCamera(false)
    this.player?.stop()
    this.player?.ctx.close()
    this.player = null
    this.ready = false
    this.set('off')
  }

  toggleMic() {
    this.micOn = !this.micOn
    if (this.micOn) this.vad?.start()
    else this.vad?.pause()
    if (this.status === 'listening' || this.status === 'hearing' || this.status === 'muted') this.toIdle()
  }

  async setCamera(on: boolean) {
    this.video?.getTracks().forEach(t => t.stop())
    this.video = null
    this.camOn = false
    if (!on) return
    try {
      this.video = await navigator.mediaDevices.getUserMedia({ video: { width: { ideal: 1280 }, height: { ideal: 720 }, facingMode: 'user' } })
      this.camOn = true
      this.camNote = ''
    } catch {
      this.camNote = 'Sin cámara: krat solo te oye.'
    }
  }

  private set(s: Status) {
    this.status = s
    this.since = performance.now()
  }

  private toIdle() {
    if (!this.ready) return
    this.vad?.setOptions({ positiveSpeechThreshold: THRESHOLD })
    this.set(this.micOn ? 'listening' : 'muted')
  }

  private connect() {
    const q = new URLSearchParams({ voice: this.voice })
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws?${q}`)
    this.ws = ws
    ws.onopen = () => { this.retry = 0; if (!this.ready) this.set('waking') }
    ws.onmessage = e => this.onMessage(JSON.parse(e.data))
    ws.onclose = () => {
      if (this.ws !== ws) return // hung up
      if (this.fatal) { // keep the explanation the server gave on screen
        const why = this.notice
        this.hangup()
        this.notice = why
        return
      }
      this.ready = false
      this.player?.stop()
      this.set('offline')
      setTimeout(() => { if (this.ws === ws) this.connect() }, Math.min(8000, 1000 * 2 ** this.retry++))
    }
  }

  private send(m: object) {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(m))
  }

  private onSpeechStart() {
    if (!this.ready) return
    // Only the server's speech detection may cut krat ('interrupted'): voices on a TV must not stop playback here.
    if (this.status === 'speaking' || this.status === 'thinking') return
    this.set('hearing')
  }

  // The audio already went out as it was spoken; the server answers on its own.
  private onSpeechEnd() {
    if (!this.ready || !this.micOn) return
    if (this.status === 'speaking') return // noise over krat's voice, not a turn
    this.sentAt = performance.now()
    this.set('thinking')
  }

  // One JPEG of what the camera sees right now, 640 px wide: enough for the model, small on the wire.
  private frame(): string {
    const v = this.videoEl
    if (!this.camOn || !v || !v.videoWidth) return ''
    const c = document.createElement('canvas')
    c.width = 640
    c.height = Math.round((640 * v.videoHeight) / v.videoWidth)
    c.getContext('2d')!.drawImage(v, 0, 0, c.width, c.height)
    return c.toDataURL('image/jpeg', 0.8).split(',')[1]
  }

  // Streams the mic as 16 kHz PCM16, one 32 ms VAD frame per message: no buffering delay.
  private feed(frame: Float32Array) {
    if (!this.ready || !this.micOn) return
    const pcm = new Int16Array(frame.length)
    for (let i = 0; i < frame.length; i++) pcm[i] = Math.max(-1, Math.min(1, frame[i])) * 0x7fff
    this.send({ type: 'audio', data: toBase64(new Uint8Array(pcm.buffer)) })
  }

  private sendFrame() {
    if (!this.ready || !this.camOn) return
    const image = this.frame()
    if (image) this.send({ type: 'frame', image })
  }

  // There are no turn ids: the server decides when a turn starts and ends.
  private onMessage(m: ServerMsg) {
    switch (m.type) {
      case 'ready':
        this.ready = true
        if (this.status === 'waking' || this.status === 'offline' || this.status === 'starting') this.toIdle()
        break
      case 'fatal':
        this.fatal = true
        this.notice = m.message ?? 'No se pudo conectar con krat.'
        break
      case 'error':
        this.notice = m.message ?? 'Se cortó la conexión con krat.'
        break
      case 'newTurn':
        this.you = this.reply = ''
        this.cut = false
        this.genDone = false
        break
      case 'transcript':
        this.you += m.text ?? ''
        break
      case 'delta':
        this.reply += m.text ?? ''
        break
      case 'audio':
        this.player?.enqueue(m.data!, () => {
          // Only once per answer. 'hearing' counts: the server may end your turn before the local VAD's 800 ms redemption.
          if (this.status !== 'thinking' && this.status !== 'listening' && this.status !== 'hearing') return
          this.latency = this.status === 'thinking' ? (performance.now() - this.sentAt) / 1000 : null
          this.vad?.setOptions({ positiveSpeechThreshold: THRESHOLD_WHILE_SPEAKING })
          this.set('speaking')
        })
        break
      case 'interrupted':
        this.player?.stop()
        if (this.reply) this.cut = true
        if (this.status === 'speaking') this.toIdle()
        break
      case 'turnDone':
        this.genDone = true
        if (!this.player?.playing && (this.status === 'thinking' || this.status === 'speaking')) this.toIdle()
        break
    }
  }
}
