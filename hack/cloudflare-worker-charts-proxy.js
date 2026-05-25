// cloudflare-worker-charts-proxy.js
//
// Source of truth for the Cloudflare Worker that powers charts.supkube.com.
// The Worker itself lives in Cloudflare's dashboard (account: Mars's Free
// plan, Worker name: charts-azure-proxy, custom domain: charts.supkube.com).
// This file is the offline mirror so we can version-control changes and
// redeploy easily if the Cloudflare account is ever recreated.
//
// Why we need this Worker (background)
// ────────────────────────────────────
// Azure Blob Static Website only serves content when both the TLS SNI and
// the HTTP Host header match the original storage account hostname
// (supkubecharts.z23.web.core.windows.net). When you front it with a
// custom domain (charts.supkube.com), the CDN/proxy in front needs to
// override Host + SNI to that storage account hostname before talking
// to Azure — otherwise Azure returns HTTP 400.
//
// Cloudflare offers this as "Origin Rules → Host Header / SNI Rewrite",
// but in 2024 they moved it behind the Enterprise plan ($200+/month).
// Workers, however, can do the rewrite in 5 lines and stay on the Free
// plan (100,000 requests/day, far more than helm install traffic).
//
// Deployment
// ──────────
// Cloudflare dashboard → Compute → Workers & Pages → charts-azure-proxy
// → Edit code → paste this file → Deploy.
//
// Then Settings → Domains & Routes → Add → Custom Domain →
// charts.supkube.com. Cloudflare signs the edge cert and routes
// charts.supkube.com traffic to this Worker.
//
// Verification
// ────────────
// curl -I https://charts.supkube.com/index.yaml
//
// Expected response:
//   HTTP/2 200
//   content-type: application/yaml
//   server: cloudflare
//   x-ms-request-id: <azure-request-id>   ← proves the Worker really did
//                                            proxy to Azure, not just
//                                            return a static cached file.

export default {
  async fetch(request) {
    // Rewrite the URL hostname to Azure Blob's storage account endpoint.
    // The Workers `fetch()` runtime sets TLS SNI based on the destination
    // hostname automatically — no separate SNI override needed.
    const url = new URL(request.url)
    url.hostname = 'supkubecharts.z23.web.core.windows.net'

    // Force the HTTP Host header to Azure's hostname so Azure routes to
    // the right storage account's $web container.
    const headers = new Headers(request.headers)
    headers.set('Host', 'supkubecharts.z23.web.core.windows.net')

    // Pass through method, body, headers. redirect: 'follow' so that
    // Azure's occasional 301s (e.g. when a customer requests a path
    // missing a trailing slash) get resolved server-side rather than
    // being returned to the helm client, which would treat them as
    // a chart-not-found.
    return fetch(url.toString(), {
      method: request.method,
      headers,
      body: request.body,
      redirect: 'follow',
    })
  },
}
