// PRD-028 / ADR-056 v2 — runtime config DEFAULT.
// Copied verbatim into dist/ by Vite (public/). At container start the nginx
// entrypoint (30-supkube-basepath.sh) OVERWRITES this file with the deployed
// basePath. This default ("/") keeps `npm run dev`, `vite preview` and a plain
// `docker run` (no configmap) working at the root path with zero 404 noise.
window.__SUPKUBE_CONFIG__ = { basePath: "/" };
