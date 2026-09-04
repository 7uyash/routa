# 📖 Routa — User Guide

This guide covers everything you need to know to get started with Routa, from CLI commands to advanced features like traffic mutation, fault simulation, webhook testing, and deterministic playback.

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [CLI Reference](#cli-reference)
3. [Web Dashboard & Inspector](#web-dashboard--inspector)
4. [Replay & Edit-Replay](#replay--edit-replay)
5. [Webhook Testing Lab](#webhook-testing-lab)
6. [Multi-Service Routing](#multi-service-routing)
7. [Traffic Mutation & Mocking](#traffic-mutation--mocking)
8. [Network Failure Simulation](#network-failure-simulation)
9. [Shadow Traffic & Response Diffing](#shadow-traffic--response-diffing)
10. [Session Storage & Deterministic Playback](#session-storage--deterministic-playback)

---

## Getting Started

### Installation

Build Routa from source using Go 1.21+:

```bash
git clone https://github.com/your-username/routa.git
cd routa
go build -o routa ./cmd/routa
```

Alternatively, add `routa` to your system `PATH`:

```bash
go install ./cmd/routa
```

---

## CLI Reference

Routa provides a clean CLI with intuitive commands and environment variable overrides.

### Commands

| Command | Description | Example |
|---------|-------------|---------|
| `routa dev <port>` | Expose local port & launch Dashboard | `routa dev 3000` |
| `routa relay` | Launch public edge Relay server | `routa relay --port 8080` |
| `routa version` | Print version and system information | `routa version` |
| `routa help` | Show CLI help text | `routa help` |

### Command Options (`routa dev`)

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--target`, `-t` | `ROUTA_TARGET` | `http://localhost:<port>` | Local target URL |
| `--dashboard-port` | `ROUTA_DASHBOARD_PORT` | `4040` | Dashboard port |
| `--relay` | `ROUTA_RELAY_URL` | `ws://localhost:8080` | Edge Relay WebSocket URL |
| `--name`, `-n` | `ROUTA_SUBDOMAIN` | Auto-generated | Requested subdomain on Relay |
| `--secret` | `ROUTA_SECRET` | `""` | Auth secret token for Relay |
| `--config`, `-c` | `ROUTA_CONFIG` | `""` | Path to YAML config (`routa.yaml`) |

### Command Options (`routa relay`)

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--port`, `-p` | `ROUTA_RELAY_PORT` | `8080` | HTTP/WS listen port |
| `--domain`, `-d` | `ROUTA_RELAY_DOMAIN` | `localhost:8080` | Base domain for agent subdomains |
| `--secret` | `ROUTA_SECRET` | `""` | Auth secret required for agent connections |

---

## Web Dashboard & Inspector

When running `routa dev`, the embedded Web Dashboard starts at `http://localhost:4040`.

### Features
- **Live Stream**: Inbound requests stream in real time over WebSockets with zero page refresh.
- **Request Filter**: Search by keyword or filter by HTTP method (`GET`, `POST`, `PUT`, `DELETE`), HTTP status (`2xx`, `4xx`, `5xx`), or request type (Normal vs Replay).
- **Split-View Detail Inspector**: Select any request to inspect:
  - **Headers**: Full raw request and response headers.
  - **Body**: Pretty-printed JSON, Form data, or plain text.
  - **Timing Breakdown**: Visual breakdown of duration and proxy processing time.
  - **Diff View**: Compare the primary response against shadow target responses.

---

## Replay & Edit-Replay

Routa allows you to re-execute any captured request without relying on external tools like curl or Postman.

1. **Replay Exact Request**: Click **Replay** on any captured request detail panel. The exact headers, query parameters, and body are re-sent.
2. **Edit & Replay**: Click **Edit & Replay** to open an interactive modal:
   - Change the HTTP method or URL path.
   - Add, edit, or delete request headers.
   - Modify the JSON payload directly in an embedded editor.
   - Click **Send Request** to execute. New replayed entries are tagged with a `Replay` badge and linked to their original parent request ID.

---

## Webhook Testing Lab

Test webhooks locally without exposing public endpoints or manually parsing provider signatures.

1. Open the **Webhook Lab** tab in the dashboard.
2. Click **Create Endpoint** to generate a unique endpoint URL (e.g. `http://<subdomain>.relay:8080/webhook/wh_abc123`).
3. Point your third-party provider (GitHub, Stripe, Shopify, Discord, Slack, etc.) to this URL.
4. Routa automatically detects the provider signature header, verifies payload structure, and logs delivery timestamps.

---

## Multi-Service Routing

Instead of proxying all traffic to a single local port, split inbound requests across multiple local microservices:

```yaml
routes:
  - path: "/api/v1/auth/*"
    target: "http://localhost:8081"
  - path: "/api/v1/users/*"
    target: "http://localhost:8082"
  - path: "/*"
    target: "http://localhost:3000"
```

You can also manage routes dynamically via the **Routes** tab in the dashboard.

---

## Traffic Mutation & Mocking

Intercept and transform HTTP traffic dynamically before it reaches your backend server.

### Supported Mutations
- **Set/Remove Headers**: Add debug flags (`X-Debug: true`) or strip authorization tokens.
- **Path Prefix Stripping**: Rewrite `/api/v1/users` to `/users`.
- **Query Parameter Injections**: Append `?debug=1&trace=true`.
- **JSON Body Dot-Path Mutations**: Mutate nested JSON fields on the fly:
  - `set: "user.role=admin"`
  - `remove: "user.ssn"`
- **Response Mocking**: Intercept specific paths and return custom JSON payloads directly without touching the backend:
  ```yaml
  request:
    mock_response:
      status: 200
      body: '{"status": "ok", "mocked": true}'
  ```

---

## Network Failure Simulation

Simulate unpredictable network conditions and test system failure recovery.

### Options
- **Fixed/Jittered Latency**: Add artificial delay (e.g. 500ms ± 100ms) to simulate slow cellular networks or remote data centers.
- **Error Injection**: Force a percentage of requests (e.g. 10%) to fail with `500 Internal Server Error` or `503 Service Unavailable`.
- **Connection Drop Rate**: Simulate abrupt TCP connection drops.
- **Timeouts**: Cancel upstream requests after a specified millisecond threshold.

Configure these live via the **Simulator** dashboard panel or through `routa.yaml`.

---

## Shadow Traffic & Response Diffing

Safely test new API versions or refactored backend services with live traffic shadowing:

```yaml
shadows:
  - name: "v2 Migration Test"
    match:
      path: "/api/v2/*"
    shadow_url: "http://localhost:9090"
    compare_response: true
```

When a request arrives at `/api/v2/*`:
1. Routa proxies the request to the primary service (`localhost:3000`).
2. Asynchronously forwards a duplicate request to the shadow service (`localhost:9090`).
3. Runs the **Deep Differ** engine comparing status codes, response headers, and nested JSON fields.
4. Renders side-by-side diff highlights in the dashboard's **Diff** tab.

---

## Session Storage & Deterministic Playback

Save traffic snapshots into JSON fixtures to share with team members or run reproducible automated tests.

1. **Save Session**: Click **Save Session** in the dashboard to persist current request history under `~/.routa/sessions/<name>.json`.
2. **Playback Session**: Run session playback via dashboard or API to re-fire recorded requests sequentially with preserved timing intervals.
