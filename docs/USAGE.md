# Routa Usage Guide

## Running Routa

### Interactive mode (recommended for first time)

Run with no arguments — Routa presents a terminal UI:

```bash
routa
```

1. Pick **Expose a local service** or **Start a relay server**
2. For dev mode: Routa scans your machine for running HTTP services and shows a list — pick one or enter a port manually
3. Fill in tunnel name, relay URL, and auth (all optional)
4. Confirm to start

---

## Dev mode — exposing a local service

```bash
routa dev <port> [flags]
```

The port can also come before or instead of `dev`:
```bash
routa 3000           # shorthand — same as routa dev 3000
routa dev            # interactive service picker
routa dev 3000       # direct
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--relay <url>` | *(empty — relay required)* | Relay WebSocket URL (`ws://` or `wss://`) |
| `--name <name>` | *(empty)* | Tunnel name used as public subdomain |
| `--dashboard <port>` | `4040` | Port for the local dashboard |
| `--token <token>` | *(empty)* | Auth token sent to the relay |
| `--auth-user <user>` | *(empty)* | Basic auth username on the public endpoint |
| `--auth-pass <pass>` | *(empty)* | Basic auth password on the public endpoint |
| `--host <host>` | `localhost` | Local hostname to forward to |
| `--max-entries <n>` | `500` | Max entries kept in the traffic inspector |

### Examples

```bash
# Simplest — tunnel port 3000
routa dev 3000 --relay ws://my-vps.com:8080

# Custom tunnel name → http://my-api.my-vps.com:8080
routa dev 3000 --relay ws://my-vps.com:8080 --name my-api

# Password-protect the public URL
routa dev 3000 --relay ws://my-vps.com:8080 --auth-user admin --auth-pass secret

# Token auth for the relay
routa dev 3000 --relay ws://my-vps.com:8080 --token relay-secret

# Custom local host (if service listens on 0.0.0.0)
routa dev 8080 --host 0.0.0.0 --relay ws://my-vps.com:8080

# Dashboard on a different port
routa dev 3000 --relay ws://my-vps.com:8080 --dashboard 5000
```

### What you get

After starting, open **http://localhost:4040** (or your `--dashboard` port) in a browser.

---

## Relay mode — running the edge server

Run this on a public VPS or server:

```bash
routa relay [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--port <port>` | `8080` | Port to listen on |
| `--host <host>` | `0.0.0.0` | Host/interface to bind to |
| `--domain <domain>` | `localhost` | Base domain — subdomains are `<name>.<domain>` |

### Examples

```bash
# Basic relay
routa relay --port 8080 --domain myserver.com:8080

# Bind to specific interface
routa relay --port 443 --host 0.0.0.0 --domain api.mycompany.com

# Local-only relay (for testing)
routa relay --port 8080 --domain localhost:8080
```

---

## Dashboard features (http://localhost:4040)

### Traffic Inspector
Live table of all requests passing through the tunnel. Click any row to see full request/response headers, body, and timing breakdown.

### Request Replay
Re-send any captured request exactly as it was. Or click **Edit & Replay** to modify the method, path, headers, or body first.

### Mutation Rules
Add rules that transform requests or responses in-flight — no code changes needed:
- Add/remove/override request or response **headers**
- **Strip** or **replace** path prefixes
- Add/remove **query parameters**
- Mutate **JSON body fields** (dot-notation path)
- Return a **mock response** (skip forwarding entirely)
- Force a specific **response status code**

### Network Simulator
Inject faults per path pattern:
- **Delay** — fixed milliseconds + random jitter
- **Error rate** — inject HTTP error status at a percentage of requests
- **Drop** — silently drop the connection (no response)
- **Timeout** — cap how long forwarding waits

### Mock Lab
Define endpoints that return fixed responses without touching your service. Useful for stubbing dependencies you don't control.

### Webhook Lab
Receive and inspect incoming webhooks. Routa detects providers (GitHub, Stripe, etc.) from headers and formats the payload.

### Shadow Traffic
Mirror every request to one or more secondary targets concurrently. Routa compares primary vs shadow responses and shows diffs in the inspector.

### Route Manager
Configure path-based routing to multiple local services:
- `/api/users/*` → `localhost:8081`
- `/api/payments/*` → `localhost:8082`
- Everything else → `localhost:3000`

### Service Discovery
Routa scans these ports on startup for active HTTP services:
`3000, 3001, 3002, 4000, 4200, 5000, 5001, 5173, 8000, 8001, 8080, 8081, 8082, 8088, 8888, 9000, 9090, 9091`

Tech stacks are inferred from response headers (Express, Next.js, FastAPI, Vite, Spring Boot, etc.).

### Session Recorder
Record a named collection of requests while you use your app, then play it back deterministically — with the same inter-request timing.

### Scenario Runner
Run multi-step request sequences:
- Template variables in paths, headers, and bodies (`{{USER_ID}}`)
- Extract values from responses into variables (JSON path, header, regex)
- Assert status codes, body content, or JSON field values
- Pass/fail report per step

---

## Config file (routa.yaml)

Place in your project root. Loaded automatically at startup.

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
  - name: "Add debug header"
    match:
      path: "/api/*"
    request:
      set_headers:
        X-Debug: "true"
      remove_headers:
        - "X-Internal-Secret"

  - name: "Block deletes"
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
    error_rate: 0.1
    error_status: 503

shadow:
  enabled: true
  targets:
    - "http://localhost:3001"

recording:
  max_entries: 1000
  redact_headers:
    - "Authorization"
    - "Cookie"
  exclude_paths:
    - "/health"
```

---

## Environment variables

| Variable | Description |
|----------|-------------|
| `ROUTA_LOCAL_PORT` | Port to forward to |
| `ROUTA_RELAY_URL` | Relay WebSocket URL |
| `ROUTA_AUTH_TOKEN` | Relay auth token |
| `ROUTA_DASHBOARD_PORT` | Dashboard port (default: `4040`) |
| `ROUTA_TUNNEL_NAME` | Tunnel name |
| `ROUTA_BASIC_AUTH_USER` | Basic auth user for public endpoint |
| `ROUTA_BASIC_AUTH_PASS` | Basic auth password |
| `ROUTA_BASE_DOMAIN` | Relay base domain |
| `ROUTA_RELAY_PORT` | Relay listen port |
| `ROUTA_DATA_DIR` | Data directory (default: `~/.routa`) |

---

## Data storage

```
~/.routa/
  sessions/     ← recorded traffic sessions (JSON)
```
