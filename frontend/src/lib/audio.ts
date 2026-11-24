// Float32 PCM (as produced by the VAD at 16 kHz) → base64 16-bit mono WAV.
export function encodeWav(samples: Float32Array, rate = 16000): string {
  const buf = new ArrayBuffer(44 + samples.length * 2)
  const v = new DataView(buf)
  const str = (o: number, s: string) => [...s].forEach((c, i) => v.setUint8(o + i, c.charCodeAt(0)))
  str(0, 'RIFF'); v.setUint32(4, 36 + samples.length * 2, true); str(8, 'WAVE')
  str(12, 'fmt '); v.setUint32(16, 16, true); v.setUint16(20, 1, true); v.setUint16(22, 1, true)
  v.setUint32(24, rate, true); v.setUint32(28, rate * 2, true); v.setUint16(32, 2, true); v.setUint16(34, 16, true)
  str(36, 'data'); v.setUint32(40, samples.length * 2, true)
  for (let i = 0; i < samples.length; i++) {
    const s = Math.max(-1, Math.min(1, samples[i]))
    v.setInt16(44 + i * 2, s < 0 ? s * 0x8000 : s * 0x7fff, true)
  }
  return toBase64(new Uint8Array(buf))
}

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
