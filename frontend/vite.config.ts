import { svelte } from '@sveltejs/vite-plugin-svelte'
import basicSsl from '@vitejs/plugin-basic-ssl'
import { defineConfig } from 'vite'
import { viteStaticCopy } from 'vite-plugin-static-copy'

const backend = process.env.BACKEND_URL ?? 'http://localhost:8080'

export default defineConfig(({ mode }) => ({
  plugins: [
    svelte(),
    // `npm run dev:lan`: other devices on the LAN need HTTPS, or browsers block mic and camera.
    mode === 'lan' && basicSsl(),
    // Silero VAD model, its worklet and the onnxruntime wasm are served locally (no CDN).
    viteStaticCopy({
      targets: [
        { src: 'node_modules/@ricky0123/vad-web/dist/vad.worklet.bundle.min.js', dest: 'vad', rename: { stripBase: true } },
        { src: 'node_modules/@ricky0123/vad-web/dist/silero_vad_v5.onnx', dest: 'vad', rename: { stripBase: true } },
        { src: 'node_modules/onnxruntime-web/dist/ort-wasm-simd-threaded.{wasm,mjs}', dest: 'vad', rename: { stripBase: true } },
        { src: 'node_modules/onnxruntime-web/dist/ort-wasm-simd-threaded.jsep.{wasm,mjs}', dest: 'vad', rename: { stripBase: true } },
      ],
    }),
  ],
  server: {
    host: mode === 'lan' ? true : undefined,
    proxy: {
      '/ws': { target: backend, ws: true },
      '/api': backend,
    },
  },
}))
