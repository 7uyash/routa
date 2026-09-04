# ⚙️ Routa — Configuration Reference

Routa can be configured using a `routa.yaml` or `routa.yml` file passed via the `--config` / `-c` flag.

```bash
routa dev --config routa.yaml
```

---

## Example `routa.yaml`

```yaml
version: "1"

agent:
  port: 4040
  target: "http://localhost:3000"
  relay_url: "ws://localhost:8080"
  subdomain: "my-service"
  secret: "my-secret-token"

routes:
  - path: "/api/v1/auth/*"
    target: "http://localhost:8081"
  - path: "/api/v1/payments/*"
    target: "http://localhost:8082"
  - path: "/*"
    target: "http://localhost:3000"

mutations:
  - name: "Inject Internal Headers"
    match:
      path: "/api/*"
      method: "POST"
    request:
      set_headers:
        X-Gateway: "Routa"
      remove_headers:
        - "X-Unwanted-Header"
      strip_path_prefix: "/api/v1"
      add_query_params:
        debug: "1"
      set_body_json:
        "metadata.processed_by": "routa-gateway"
      remove_body_json:
        - "internal_ssn"

  - name: "Mock Maintenance Status"
    match:
      path: "/api/v1/maintenance"
    request:
      mock_response:
        status: 503
        body: '{"error": "Service under scheduled maintenance"}'

simulations:
  - name: "Payment Latency & Error Injection"
    match:
      path: "/api/v1/payments/*"
      method: "POST"
    latency_ms: 300
    jitter_ms: 50
    error_rate: 0.05
    error_status: 500
    drop_rate: 0.01

shadows:
  - name: "Search Engine Migration Shadow"
    match:
      path: "/api/v1/search"
    shadow_url: "http://localhost:9090"
    compare_response: true
```

---

## Schema Specification

### 1. `agent` Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | `int` | `4040` | Port for the embedded Web Dashboard REST API & UI. |
| `target` | `string` | `http://localhost:3000` | Default HTTP target URL for proxied requests. |
| `relay_url` | `string` | `ws://localhost:8080` | Edge Relay WebSocket endpoint URL. |
| `subdomain` | `string` | `""` | Desired subdomain requested on the edge Relay. |
| `secret` | `string` | `""` | Authentication secret token for Relay connection. |

---

### 2. `routes` Section

List of pattern-matching route rules evaluated in sequential order:

| Field | Type | Description |
|-------|------|-------------|
| `path` | `string` | URL path pattern (`/exact`, `/prefix/*`, `/*`). |
| `target` | `string` | Target backend HTTP base URL (e.g. `http://localhost:8081`). |

---

### 3. `mutations` Section

List of request/response mutation and mock rules:

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Descriptive label for the mutation rule. |
| `match.path` | `string` | URL path pattern to match. |
| `match.method` | `string` | HTTP method filter (`GET`, `POST`, `*`). |
| `request.set_headers` | `map[string]string` | Key-value pairs of request headers to inject/override. |
| `request.remove_headers` | `[]string` | List of header keys to strip from request. |
| `request.strip_path_prefix` | `string` | Path prefix to strip before forwarding to target. |
| `request.add_query_params` | `map[string]string` | Query string key-values to append to URL. |
| `request.set_body_json` | `map[string]string` | Dot-path key-value assignments for JSON body (`"user.role": "admin"`). |
| `request.remove_body_json` | `[]string` | Dot-path JSON body keys to delete (`"user.ssn"`). |
| `request.mock_response.status` | `int` | Force HTTP response status code without contacting target. |
| `request.mock_response.body` | `string` | Return raw JSON/string response body. |

---

### 4. `simulations` Section

List of chaos & fault injection simulation rules:

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Descriptive label for the simulation rule. |
| `match.path` | `string` | URL path pattern to match. |
| `match.method` | `string` | HTTP method filter. |
| `latency_ms` | `int` | Base artificial delay in milliseconds. |
| `jitter_ms` | `int` | Random variance range for latency in milliseconds. |
| `error_rate` | `float` | Probability of returning simulated error (`0.0` to `1.0`). |
| `error_status` | `int` | HTTP status code for generated errors (default `500`). |
| `drop_rate` | `float` | Probability of terminating connection immediately (`0.0` to `1.0`). |
| `timeout_ms` | `int` | Maximum request duration before timing out. |

---

### 5. `shadows` Section

List of shadow traffic rules:

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Descriptive label for shadow test. |
| `match.path` | `string` | URL path pattern to match. |
| `shadow_url` | `string` | Secondary HTTP target base URL to receive duplicated traffic. |
| `compare_response` | `bool` | Whether to run deep response differ and record comparison results. |
