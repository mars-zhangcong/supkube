import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { readFileSync } from 'node:fs'

// Read version from package.json so we have ONE source of truth.
// Bump package.json on every release and the header chip updates
// automatically — no more "shows step3 but actually step6" confusion.
const pkg = JSON.parse(readFileSync(new URL('./package.json', import.meta.url)))

// Build stamp = "YYMMDD-HHMM" of the moment vite builds.
// Lets the user eyeball "am I looking at a fresh build?" in the UI without
// having to inspect bundle hashes. We DON'T use git rev-parse here because
// Docker builds copy files in and may not have .git available.
const buildStamp = (() => {
  const d = new Date()
  const p = (n) => String(n).padStart(2, '0')
  return `${String(d.getFullYear()).slice(2)}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}`
})()

export default defineConfig({
  plugins: [vue()],
  // __SUPKUBE_VERSION__ and __SUPKUBE_BUILD__ are inlined at build time.
  // src/version.js re-exports them so component code doesn't have to know
  // about the build-time substitution mechanism.
  define: {
    __SUPKUBE_VERSION__: JSON.stringify(pkg.version),
    __SUPKUBE_BUILD__: JSON.stringify(buildStamp)
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
})
