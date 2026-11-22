<script lang="ts">
  import type { Status } from './call.svelte'

  let { status, level }: { status: Status; level: () => number } = $props()

  // Vibration modes (n, m, sign) per state. Sand settles on the nodal lines of
  // f = cos(nπx)cos(mπy) ± cos(mπx)cos(nπy); changing mode makes it migrate.
  // The "−" family always has the x = y diagonal as a node, so the "+" family keeps figures varied.
  type Mode = [number, number, 1 | -1]
  const MODES: Record<Status, Mode[]> = {
    off: [[1, 3, -1]],
    starting: [[1, 3, -1]],
    waking: [[1, 2, -1]],
    offline: [[1, 2, -1]],
    muted: [[1, 3, 1]],
    listening: [[2, 4, 1]],
    hearing: [[2, 5, 1]],
    thinking: [[3, 5, 1], [4, 7, -1], [2, 7, 1], [5, 6, -1]],
    speaking: [[3, 4, 1], [4, 5, -1], [3, 7, 1], [2, 5, -1]],
  }
  // How hard the plate is driven before the live audio level is added.
  const DRIVE: Record<Status, number> = {
    off: 0.35, starting: 0.5, waking: 0.25, offline: 0.12, muted: 0.2,
    listening: 0.4, hearing: 0.45, thinking: 0.7, speaking: 0.6,
  }
  const CYCLE_MS: Partial<Record<Status, number>> = { thinking: 1600, speaking: 3000 }
  const SAND = '#EAE3D2'
  const BRASS = '#C9A44C'

  let canvas: HTMLCanvasElement

  $effect(() => {
    const ctx = canvas.getContext('2d')!
    const reduce = matchMedia('(prefers-reduced-motion: reduce)')
    const N = 3200
    const xs = new Float32Array(N).map(() => Math.random())
    const ys = new Float32Array(N).map(() => Math.random())
    let size = 0
    let dpr = 1
    let mode: Mode = MODES.off[0]
    let modeAt = 0
    let lastStatus: Status | null = null
    let raf = 0
    let dirty = true

    const ro = new ResizeObserver(([e]) => {
      dpr = Math.min(2, devicePixelRatio || 1)
      size = e.contentRect.width
      canvas.width = canvas.height = Math.round(size * dpr)
    })
    ro.observe(canvas)

    const f = (x: number, y: number) => {
      const [n, m, sign] = mode
      return Math.cos(n * Math.PI * x) * Math.cos(m * Math.PI * y) + sign * Math.cos(m * Math.PI * x) * Math.cos(n * Math.PI * y)
    }

    // Sand is kicked in proportion to how much its spot vibrates, so it drifts to the still lines.
    const step = (drive: number) => {
      const k = 0.05 * drive
      for (let i = 0; i < N; i++) {
        const a = Math.abs(f(xs[i], ys[i])) * k + 0.00015
        xs[i] = Math.min(1, Math.max(0, xs[i] + (Math.random() - 0.5) * a))
        ys[i] = Math.min(1, Math.max(0, ys[i] + (Math.random() - 0.5) * a))
      }
    }

    const draw = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height)
      ctx.fillStyle = status === 'hearing' ? BRASS : SAND
      ctx.globalAlpha = 0.4 // grains piling up on a nodal line read brighter than stray sand
      const s = 2 * dpr
      const w = canvas.width - s
      for (let i = 0; i < N; i++) ctx.fillRect(xs[i] * w, ys[i] * w, s, s)
    }

    const pickMode = (t: number) => {
      const list = MODES[status]
      const cycle = CYCLE_MS[status]
      if (status !== lastStatus) {
        lastStatus = status
        mode = list[0]
        modeAt = t
        return true
      }
      if (cycle && !reduce.matches && t - modeAt > cycle) {
        mode = list[(list.indexOf(mode) + 1) % list.length]
        modeAt = t
      }
      return false
    }

    const frame = (t: number) => {
      raf = requestAnimationFrame(frame)
      const changed = pickMode(t)
      if (reduce.matches) {
        // No continuous motion: settle each new figure at once and hold it.
        if (changed) {
          for (let i = 0; i < 300; i++) step(1)
          dirty = true
        }
        if (!dirty || !size) return
        dirty = false
      } else {
        // Sub-steps: figures must form fast enough to read between mode changes. More drive means
        // faster settling, not more noise: sand near a nodal line barely moves at any drive.
        const drive = DRIVE[status] + level() * 0.8
        for (let i = 0; i < 4; i++) step(drive)
        if (!size) return
      }
      draw()
    }
    for (let i = 0; i < 400; i++) step(1) // start from a settled figure, not noise
    raf = requestAnimationFrame(frame)

    return () => {
      cancelAnimationFrame(raf)
      ro.disconnect()
    }
  })
</script>

<div class="plate">
  <canvas bind:this={canvas} aria-hidden="true"></canvas>
</div>

<style>
  .plate {
    width: var(--plate-size);
    aspect-ratio: 1;
    padding: clamp(10px, 2.4%, 16px);
    border-radius: 10px;
    background: var(--steel-plate);
    box-shadow: inset 0 0 0 1px var(--steel-line);
  }
  canvas {
    display: block;
    width: 100%;
    height: 100%;
  }
</style>
