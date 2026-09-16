# Routa — Developer Traffic Gateway

Routa exposes your local HTTP service to the internet through a WebSocket tunnel, with a built-in dashboard, traffic inspector, mutation engine, fault simulator, shadow traffic, and mock lab — all from a single binary.

---

## How it works

```
  Browser / API client
         |
  [Relay Server]  ←── runs on a public machine (VPS / cloud)
         |  WebSocket tunnel
  [Routa Agent]   ←── runs on your laptop
         |
  localhost:3000   ←── your local service
```

The **relay** is a lightweight edge server you self-host. The **agent** connects to it and forwards all incoming HTTP requests to your local service. There is no managed cloud — you host both ends.

---

## Installation

### Pre-built binaries

| Platform | Binary |
|----------|--------|
| macOS (Apple Silicon) | `bin/routa-darwin-arm64` |
| macOS (Intel) | `bin/routa-darwin-amd64` |
| Linux ARM64 | `bin/routa-linux-arm64` |
| Linux AMD64 | `bin/routa-linux-amd64` |
| Windows AMD64 | `bin/routa-windows-amd64.exe` |
| Windows ARM64 | `bin/routa-windows-arm64.exe` |

**macOS / Linux:**
```bash
git clone https://github.com/7uyash/routa.git
cd routa
chmod +x bin/routa-darwin-arm64        # or your platform binary
sudo mv bin/routa-darwin-arm64 /usr/local/bin/routa
```

**Windows (PowerShell):**
```powershell
git clone https://github.com/7uyash/routa.git
cd routa
# Run directly:
.\bin\routa-windows-amd64.exe dev 3000
# Or install globally:
Copy-Item .\bin\routa-windows-amd64.exe C:\Windows\System32\routa.exe
```

### Build from source
```bash
# Requires Go 1.21+
go build -o routa ./cmd/routa

# Cross-compile all platforms
make build-all          # Linux / macOS
.\build.ps1             # Windows
```

---

## Quick start

### Step 1 — Start the relay (on a public server)
```bash
routa relay --port 8080 --domain myserver.com:8080
```

### Step 2 — Expose your local service (on your laptop)
```bash
routa dev 3000 --relay ws://myserver.com:8080 --name my-app
```

Your service is now reachable at `http://my-app.myserver.com:8080`.  
Dashboard: `http://localhost:4040`

---

## Usage

### Interactive mode (no arguments)
```bash
routa
```
Shows a terminal UI where you:
1. Choose **Expose a local service** or **Start a relay server**
2. For dev mode — auto-discovers running local services and lets you pick one
3. Configures tunnel name, relay URL, and auth interactively

### Dev mode (expose a local service)
```bash
routa dev <port> [flags]

# Shorthand — port only:
routa 3000
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--relay <url>` | *(required unless set via env)* | Relay WebSocket URL, e.g. `ws://myserver.com:8080` |
| `--name <name>` | *(random)* | Tunnel name / public subdomain |
| `--dashboard <port>` | `4040` | Local dashboard port |
| `--token <token>` | — | Auth token for the relay |
| `--auth-user <user>` | — | Basic auth username on the public endpoint |
| `--auth-pass <pass>` | — | Basic auth password on the public endpoint |
| `--host <host>` | `localhost` | Local host to forward to |
| `--max-entries <n>` | `500` | Max requests kept in traffic inspector |

**Examples:**
```bash
routa dev 3000
routa dev 8080 --name api --relay ws://relay.example.com:8080
routa dev 5173 --token secret123
routa dev 3000 --auth-user admin --auth-pass secret  # protect the public URL
```

### Relay mode (run the edge server)
```bash
routa relay [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--port <port>` | `8080` | Port to listen on |
| `--host <host>` | `0.0.0.0` | Host to bind to |
| `--domain <domain>` | `localhost` | Base domain for subdomains |

**Example:**
```bash
routa relay --port 8080 --domain relay.example.com:8080
```

### Other commands
```bash
routa version     # Print version (v0.1.0)
routa help        # Print usage
routa --help
routa -h
```

---

## Config file (routa.yaml)

Place `routa.yaml` in your project directory. It is automatically loaded at startup and overrides CLI flags.

```yaml
tunnel:
  port: 3000
  relay_url: "ws://myserver.com:8080"
  name: "my-app"
  dashboard_port: 4040
  auth_token: ""
  basic_auth_user: ""
  basic_auth_pass: ""

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

## Dashboard (http://localhost:4040)

The local dashboard gives you:

- **Traffic Inspector** — Live feed of all requests/responses with headers, bodies, and timing
- **Replay** — Re-send any captured request; edit method/path/headers/body before replaying
- **Mutation Rules** — Add/edit/remove request & response mutation rules at runtime
- **Network Simulator** — Inject latency, jitter, error rates, and connection drops per path
- **Mock Lab** — Define HTTP endpoints that return fixed responses without touching your service
- **Webhook Lab** — Receive and inspect incoming webhooks
- **Shadow Traffic** — Mirror traffic to secondary targets and diff responses
- **Service Discovery** — See all detected local services and their tech stack
- **Route Manager** — Configure path-based routing to multiple local services
- **Session Recorder** — Record request collections and play them back deterministically
- **Scenario Runner** — Run multi-step request sequences with variable extraction and assertions

---

## Environment variables

All config can be set via environment variables:

| Variable | Config field |
|----------|-------------|
| `ROUTA_LOCAL_PORT` | Local port to forward to |
| `ROUTA_RELAY_URL` | Relay WebSocket URL |
| `ROUTA_AUTH_TOKEN` | Auth token |
| `ROUTA_DASHBOARD_PORT` | Dashboard port |
| `ROUTA_TUNNEL_NAME` | Tunnel name |
| `ROUTA_BASIC_AUTH_USER` | Basic auth username |
| `ROUTA_BASIC_AUTH_PASS` | Basic auth password |
| `ROUTA_BASE_DOMAIN` | Relay base domain |
| `ROUTA_RELAY_PORT` | Relay listen port |
| `ROUTA_DATA_DIR` | Data directory (default: `~/.routa`) |

---

## Local service discovery

When running in interactive mode (`routa` or `routa dev` with no port), Routa scans these ports for active HTTP services:

`3000, 3001, 3002, 4000, 4200, 5000, 5001, 5173, 8000, 8001, 8080, 8081, 8082, 8088, 8888, 9000, 9090, 9091`

It identifies tech stacks from response headers (Express, Next.js, FastAPI, Spring Boot, Vite, etc.) and shows them in a pick-list.

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