export function toBase64(bytes: Uint8Array): string {
  let bin = ''
  for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  return btoa(bin)
}

// RMS of an analyser's current window, scaled to roughly 0..1 for speech.
export function level(an: AnalyserNode, scratch: Float32Array<ArrayBuffer>): number {
  an.getFloatTimeDomainData(scratch)
  let sum = 0
  for (const s of scratch) sum += s * s
  return Math.min(1, Math.sqrt(sum / scratch.length) * 4)
}

// Plays krat's voice as 24 kHz 16-bit PCM chunks streamed from the backend, gapless,
// and exposes the output level for the plate.
const RATE = 24000

export class Player {
  readonly ctx = new AudioContext()
  readonly analyser = this.ctx.createAnalyser()
  private scratch = new Float32Array(1024)
  private sources = new Set<AudioBufferSourceNode>()
  private at = 0
  onDrain = () => {}

  constructor() {
    this.analyser.fftSize = 1024
    this.analyser.connect(this.ctx.destination)
  }

  get playing() { return this.sources.size > 0 }

  level() { return level(this.analyser, this.scratch) }

  enqueue(b64pcm: string, onStart?: () => void) {
    const bin = atob(b64pcm)
    const n = bin.length >> 1
    if (!n) return
    const buf = this.ctx.createBuffer(1, n, RATE)
    const ch = buf.getChannelData(0)
    for (let i = 0; i < n; i++) {
      const v = bin.charCodeAt(2 * i) | (bin.charCodeAt(2 * i + 1) << 8)
      ch[i] = (v >= 0x8000 ? v - 0x10000 : v) / 0x8000
    }
    const src = this.ctx.createBufferSource()
    src.buffer = buf
    src.connect(this.analyser)
    // A little head start on the first chunk absorbs network jitter between chunks.
    const start = Math.max(this.ctx.currentTime + 0.1, this.at)
    this.at = start + buf.duration
    src.onended = () => {
      this.sources.delete(src)
      if (!this.playing) this.onDrain()
    }
    this.sources.add(src)
    src.start(start)
    if (onStart) setTimeout(onStart, (start - this.ctx.currentTime) * 1000)
  }

  stop() {
    for (const s of this.sources) { s.onended = null; s.stop() }
    this.sources.clear()
    this.at = 0
  }
}
