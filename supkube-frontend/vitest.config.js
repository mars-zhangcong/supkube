// Vitest config (PRD-010 / introduced for DRTopology TC-TOPO-001..005).
//
// We keep this separate from vite.config.js so Vitest doesn't drag the
// production define() build stamp + i18n bootstrap into the test bundle.
// `test.environment: jsdom` is required for components that read
// document / window (DRTopology mounts with localStorage / classList).

import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.test.{js,ts}'],
    // Don't fail the run on console.warn from the unknown-flow-type fallback
    // path (we explicitly assert that warning in TC-TOPO-002b).
  },
})
