# LegacyStore Web

The browser UI is server-rendered by the Go backend in backend/internal/webui.

nginx is intentionally only the public HTTP/TLS reverse proxy. It does not
render or serve a separate SPA bundle anymore. HTML templates, the Safari-5
baseline stylesheet and the small ES5 progressive-enhancement script are
embedded into the Go binary.

Public browser flow:

    Browser -> nginx TLS/reverse proxy -> Go webui -> catalog/account stores -> PostgreSQL

The REST API remains available under /api/v1/ for the native legacy client,
automation and integrations.

The SSR baseline is designed to remain functional without JavaScript. The
embedded JavaScript only adds non-essential confirmation behavior.
