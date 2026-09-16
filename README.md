# Routa — Developer Traffic Gateway

Routa exposes your local HTTP service to the internet through a WebSocket tunnel, with a built-in dashboard, traffic inspector, mutation engine, fault simulator, shadow traffic, and mock lab — all from a single binary.

---

## ⚡ Quick Start (The Shortest Way)

Install the latest version using Go:
```powershell
go install github.com/7uyash/routa/cmd/routa@latest
```

Start Routa locally (no target required at startup):
```powershell
routa dev
```
*(This instantly launches the Dashboard in your browser where you can pick or enter a target dynamically!)*

If you already know the port you want to proxy (e.g., 3000):
```powershell
routa dev 3000
```

---

## How it works

```
  Browser / API client
         |
  [Routa Local Proxy]   ←── http://localhost:4000 (Your local entry point)
         |
  [Routa Agent]         ←── Logs, Inspects, Mutates, Simulates
         |
  [Local Service]       ←── localhost:3000 (Your app)
```

Routa is designed to make local API testing flawless. When you run `routa dev`, it spins up:
1. **A Dashboard (`http://localhost:4040`)** — To configure rules, mock endpoints, and monitor traffic in real time.
2. **A Dedicated Local Proxy (`http://localhost:4000`)** — Your new entry point. Access this URL, and Routa forwards everything accurately to your local application, allowing you to intercept traffic even for root routes (`/`).

---

## Features

- **Dynamic Target Selection** — Change your proxy destination straight from the UI without restarting the terminal process.
- **Service Discovery** — Instantly discovers services running on local ports and auto-detects their tech stack (Vite, Next.js, Express, FastAPI, etc.).
- **Traffic Inspector** — Live feed of all requests/responses with headers, bodies, and timing.
- **Replay** — Re-send any captured request; edit method/path/headers/body before replaying.
- **Mutation Rules** — Add/edit/remove request & response mutation rules at runtime.
- **Network Simulator** — Inject latency, jitter, error rates, and connection drops per path.
- **Mock Lab** — Define HTTP endpoints that return fixed responses without touching your service.
- **Webhook Lab** — Receive and inspect incoming webhooks.
- **Shadow Traffic** — Mirror traffic to secondary targets and diff responses.
- **Session Recorder** — Record request collections and play them back deterministically.

---

## Usage

### Local Proxy Mode (Dev Mode)
```bash
routa dev <port> [flags]

# Start and pick target from the Dashboard
routa dev
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--relay <url>` | — | Relay WebSocket URL, e.g. `ws://myserver.com:8080` |
| `--name <name>` | *(random)* | Tunnel name / public subdomain |
| `--dashboard <port>` | `4040` | Local dashboard port |
| `--proxy <port>` | `4000` | Local proxy port |
| `--token <token>` | — | Auth token for the relay |
| `--auth-user <user>` | — | Basic auth username on the public endpoint |
| `--auth-pass <pass>` | — | Basic auth password on the public endpoint |
| `--host <host>` | `localhost` | Local host to forward to |
| `--max-entries <n>` | `500` | Max requests kept in traffic inspector |

### Public Tunnel Relay Mode

Want to share your local environment with someone else or hook it up to external webhooks? You can spin up a lightweight, self-hosted edge server:

**Step 1 — Start the relay (on a public server)**
```bash
routa relay --port 8080 --domain myserver.com:8080
```

**Step 2 — Expose your local service (on your laptop)**
```bash
routa dev 3000 --relay ws://myserver.com:8080 --name my-app
```
Your service is now securely reachable at `http://my-app.myserver.com:8080`.

---

## Config file (routa.yaml)

Place `routa.yaml` in your project directory. It is automatically loaded at startup and overrides CLI flags.

```yaml
tunnel:
  port: 3000
  relay_url: "ws://myserver.com:8080"
  name: "my-app"
  dashboard_port: 4040
  proxy_port: 4000
  auth_token: ""

routes:
  - pattern: "/api/users/*"
    target: "http://localhost:8081"
    name: "users-service"
  - pattern: "/api/payments/*"
    target: "http://localhost:8082"
    name: "payments-service"

mutations:
  - name: "Inject debug header"
    match:
      path: "/api/*"
      method: "GET"
    request:
      set_headers:
        X-Debug: "true"
      remove_headers:
        - "X-Internal-Token"
      strip_path_prefix: "/api/v1"
      set_query:
        debug: "true"
      remove_query:
        - "secret"
      set_body_fields:
        "user.role": '"admin"'
    response:
      set_headers:
        X-Served-By: "routa"
      force_status: 200

  - name: "Block DELETE requests"
    match:
      method: "DELETE"
    response:
      mock_status: 403
      mock_body: '{"error":"not allowed"}'
      mock_headers:
        Content-Type: "application/json"

simulations:
  - name: "Slow payments"
    match:
      path: "/api/payments/*"
    delay_ms: 300
    jitter_ms: 50
    error_rate: 0.05
    error_status: 503
    drop: false

shadow:
  enabled: true
  targets:
    - "http://localhost:3001"

recording:
  enabled: true
  max_entries: 1000
  redact_headers:
    - "Authorization"
    - "Cookie"
  redact_body_fields:
    - "user.password"
  exclude_paths:
    - "/health"
    - "/metrics"
```

---

## Environment variables

All config can be set via environment variables:

| Variable | Config field |
|----------|-------------|
| `ROUTA_LOCAL_PORT` | Local port to forward to |
| `ROUTA_RELAY_URL` | Relay WebSocket URL |
| `ROUTA_AUTH_TOKEN` | Auth token |
| `ROUTA_DASHBOARD_PORT` | Dashboard port |
| `ROUTA_PROXY_PORT` | Local proxy port |
| `ROUTA_TUNNEL_NAME` | Tunnel name |
| `ROUTA_BASIC_AUTH_USER` | Basic auth username |
| `ROUTA_BASIC_AUTH_PASS` | Basic auth password |
| `ROUTA_BASE_DOMAIN` | Relay base domain |
| `ROUTA_RELAY_PORT` | Relay listen port |
| `ROUTA_DATA_DIR` | Data directory (default: `~/.routa`) |

---

## Installation via Source

```bash
# Requires Go 1.21+
git clone https://github.com/7uyash/routa.git
cd routa
go build -o routa ./cmd/routa

# Cross-compile all platforms
make build-all          # Linux / macOS
.\build.ps1             # Windows
```

---

## Data storage

Sessions and scenarios are stored in `~/.routa/` (or `ROUTA_DATA_DIR`).

```
~/.routa/
  sessions/   ← recorded traffic sessions
```

---

## Testing

```bash
go test ./...
go vet ./...
```

---

## License

MIT