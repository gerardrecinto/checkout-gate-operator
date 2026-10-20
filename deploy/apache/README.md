# Apache in front of the dashboard and status API

`checkout-gate.conf` is a real Apache vhost that fixes a gap in how this
project ships today: the Dockerfile builds the Svelte dashboard and
copies the compiled static files into the image at `/frontend-dist`,
but nothing in the Go binary (`internal/apiserver`) ever serves them -
it only registers `/api/gates`, `/api/gates/summary`, and `/healthz`.
Without something in front of it serving those static files, the
container as shipped can't actually deliver the dashboard; the demo
GIF in the top-level README was captured against `npm run dev`, not
the packaged image.

This config is that missing piece, and it's also the natural fit for
Apache specifically over a pure load balancer like HAProxy: Apache can
serve the dashboard's static files directly *and* reverse-proxy API
calls to the manager, in one vhost, one process.

## What it does

- Serves `frontend/dist` (or `/frontend-dist` inside the container) as
  static files at `/`.
- Falls back to `index.html` for any path that isn't a real file and
  isn't `/api/*` or `/healthz`, the same idea as an nginx `try_files`
  fallback, so client-side routing in the Svelte app works on a hard
  refresh.
- Reverse-proxies `/api/*` and `/healthz` to the manager's status API
  (`internal/apiserver`, `:8081` by default).

## What's verified, and how

- Config syntax: `httpd -t -f <this file included from a full config>`
  reports `Syntax OK` against a real Apache 2.4.67 install.
- Static serving and the SPA fallback rewrite: run for real against a
  live `httpd` process locally, `curl` against both `/` and an
  arbitrary unknown path both returned `HTTP 200` served from
  `index.html`, confirming the `<Directory>`/`RewriteRule` block
  behaves as intended.
- The `ProxyPass`/`ProxyPassReverse` directives: syntax-checked only,
  not run live end to end. Loading `mod_proxy`/`mod_proxy_http` at all
  reproducibly prevented `httpd` from starting on the specific macOS
  Apache build used to test this (confirmed by isolating it down to
  just the two `LoadModule` lines with zero `ProxyPass` usage, no
  config content involved) - a local build/sandbox limitation on that
  one machine, not something specific to Linux production Apache
  installs, where `mod_proxy_http` is one of the most widely deployed
  Apache modules there is. Said here plainly rather than claimed as
  fully tested when it wasn't.

## Using it

```bash
# Debian/Ubuntu
sudo a2enmod proxy proxy_http rewrite headers
sudo cp checkout-gate.conf /etc/apache2/sites-available/
sudo a2ensite checkout-gate
sudo systemctl reload apache2

# Point DocumentRoot at wherever your build actually puts frontend/dist
# if you're not running this inside the project's own container image.
```

Add TLS termination with `mod_ssl` and a `<VirtualHost *:443>` block in
front of this same setup for production; this file is deliberately kept
to plain HTTP to stay focused on the proxy/static-serving pattern
itself, which is the part this repo was actually missing.
