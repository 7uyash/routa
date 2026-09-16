# Routa Configuration Reference

All configuration comes from three sources, applied in this priority order:
1. **Environment variables** (highest)
2. **CLI flags**
3. **`routa.yaml`** (project config file)
4. **Defaults** (lowest)

---

## CLI flags

### Dev mode (`routa dev <port>`)

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--relay <url>` | string | `""` | Relay WebSocket URL (`ws://` or `wss://`) |
| `--name <name>` | string | `""` | Tunnel name / public subdomain |
| `--dashboard <port>` | int | `4040` | Local dashboard port |
| `--token <token>` | string | `""` | Auth token for the relay |
| `--auth-user <user>` | string | `""` | Basic auth username on the public endpoint |
| `--auth-pass <pass>` | string | `""` | Basic auth password on the public endpoint |
| `--host <host>` | string | `localhost` | Local host to forward to |
| `--max-entries <n>` | int | `500` | Max traffic inspector entries |

### Relay mode (`routa relay`)

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--port <port>` | int | `8080` | Port to listen on |
| `--host <host>` | string | `0.0.0.0` | Host/interface to bind |
| `--domain <domain>` | string | `localhost` | Base domain for subdomains |

---

## Environment variables

| Variable | Type | Description |
|----------|------|-------------|
| `ROUTA_LOCAL_PORT` | int | Port of the local service to tunnel |
| `ROUTA_RELAY_URL` | string | Relay WebSocket URL |
| `ROUTA_AUTH_TOKEN` | string | Auth token for the relay |
| `ROUTA_DASHBOARD_PORT` | int | Dashboard listen port |
| `ROUTA_TUNNEL_NAME` | string | Tunnel name / subdomain |
| `ROUTA_BASIC_AUTH_USER` | string | Basic auth username |
| `ROUTA_BASIC_AUTH_PASS` | string | Basic auth password |
| `ROUTA_BASE_DOMAIN` | string | Relay base domain |
| `ROUTA_RELAY_PORT` | int | Relay listen port |
| `ROUTA_DATA_DIR` | string | Data directory (default: `~/.routa`) |

---

## routa.yaml schema

```yaml
tunnel:
  port: 3000                          # int — local service port
  relay_url: "ws://host:8080"         # string — relay WebSocket URL
  name: "my-app"                      # string — tunnel name / subdomain
  dashboard_port: 4040                # int — local dashboard port
  auth_token: ""                      # string — relay auth token
  basic_auth_user: ""                 # string — public endpoint basic auth user
  basic_auth_pass: ""                 # string — public endpoint basic auth pass

routes:
  - pattern: "/api/users/*"           # string — path glob
    target: "http://localhost:8081"   # string — local target URL
    name: "users-service"             # string — optional label

mutations:
  - name: "rule name"                 # string
    match:
      path: "/api/*"                  # string — path glob, empty = all
      method: "GET"                   # string — HTTP method, empty = all
    request:
      set_headers:                    # map[string]string
        X-My-Header: "value"
      remove_headers:                 # []string
        - "Authorization"
      strip_path_prefix: "/api/v1"    # string — strip this prefix from path
      replace_path: "/new/path"       # string — replace entire path
      set_query:                      # map[string]string
        debug: "true"
      remove_query:                   # []string
        - "token"
      set_body_fields:                # map[string]string — dot-notation → JSON value
        "user.role": '"admin"'
    response:
      set_headers:                    # map[string]string
        X-Served-By: "routa"
      remove_headers:                 # []string
        - "X-Internal"
      force_status: 200               # int — override response status code
      mock_status: 403                # int — return this status without forwarding
      mock_body: '{"error":"no"}'     # string — mock response body
      mock_headers:                   # map[string]string — mock response headers
        Content-Type: "application/json"

simulations:
  - name: "slow payments"             # string
    match:
      path: "/api/payments/*"         # string — path glob
      method: ""                      # string — HTTP method, empty = all
    delay_ms: 300                     # int — fixed delay in milliseconds
    jitter_ms: 50                     # int — random ±jitter added to delay
    bandwidth_bps: 0                  # int — throttle bytes/sec, 0 = unlimited
    error_rate: 0.1                   # float — fraction of requests to inject error (0.0–1.0)
    error_status: 503                 # int — status code to inject
    timeout_ms: 0                     # int — kill forwarding after this many ms
    drop: false                       # bool — silently drop connection (no response)

shadow:
  enabled: true                       # bool
  targets:                            # []string
    - "http://localhost:3001"

recording:
  enabled: true                       # bool
  max_entries: 500                    # int — max entries in traffic inspector
  redact_headers:                     # []string — headers to blank out in recordings
    - "Authorization"
    - "Cookie"
  redact_body_fields:                 # []string — dot-path JSON fields to blank out
    - "user.password"
  exclude_paths:                      # []string — paths never recorded
    - "/health"
    - "/metrics"
```

---

## Path matching

Paths use suffix glob matching:
- `/api/*` — matches `/api/users`, `/api/users/123`, etc.
- `/health` — exact match only
- `""` (empty) — matches all paths

---

## Mutation rule evaluation

- Rules are evaluated **in order** (first match wins for mock responses; all matching rules apply for header mutations)
- `mock_status` on a response rule **short-circuits** forwarding — the request never reaches your local service
- Request mutations are applied **before** forwarding; response mutations after

---

## Data directory

Default: `~/.routa/`

```
~/.routa/
  sessions/     ← recorded traffic sessions (JSON files)
```

Override with `ROUTA_DATA_DIR` or the `data_dir` config field.
