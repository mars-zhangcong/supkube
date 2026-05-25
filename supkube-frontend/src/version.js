// version.js — single source of truth for the app version string.
//
// Why this file exists:
//   Until v0.8.5-step5 the version was hard-coded in two places
//   (App.vue header chip + Login.vue footer). They drifted —
//   the header still showed `v0.8.5-alpha-step3` long after we'd
//   shipped step 4/5/6. Users couldn't tell if their browser had a
//   stale bundle or if a deploy hadn't landed.
//
// How it works now:
//   - `package.json#version` is the canonical version
//   - `vite.config.js` reads it + a build timestamp and injects two
//     constants via `define`: __SUPKUBE_VERSION__, __SUPKUBE_BUILD__
//   - this module exposes them as named exports
//
// Bump procedure:
//   1. Edit package.json → "version": "0.8.5-alpha-step7"
//   2. npm run build  (or docker build — Dockerfile runs vite build)
//   3. New bundle has the new chip; old bundles in browser cache will
//      show the old version, which is the SIGNAL the user needs.

/* global __SUPKUBE_VERSION__, __SUPKUBE_BUILD__ */
export const VERSION = typeof __SUPKUBE_VERSION__ === 'string' ? __SUPKUBE_VERSION__ : 'dev'
export const BUILD = typeof __SUPKUBE_BUILD__ === 'string' ? __SUPKUBE_BUILD__ : 'local'

// Convenience composite, e.g. "v0.8.5-alpha-step6 · 260522-2310".
// Use this when you want the user to be able to spot a stale cache.
export const FULL_VERSION = `v${VERSION} · ${BUILD}`
