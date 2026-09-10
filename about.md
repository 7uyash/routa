# 🌐 About Routa

**Routa** (pronounced */ru-ta/*) is an all-in-one **High-Performance Developer Traffic Gateway, Local Tunneling System, HTTP Inspector, and Chaos Engineering Toolkit** written in Go.

It sits as an intelligent layer between incoming network traffic (frontend applications, public webhooks, remote clients) and your local backend microservices. Routa gives developers **100% real-time visibility, total traffic control, and advanced simulation capabilities** over every single HTTP request and response — all through a single, lightweight binary and an embedded web dashboard.

---

## 🎯 What Problem Does Routa Solve?

During backend and API development, developers often have to stitch together multiple separate tools:
- **Ngrok / Localtunnel** to expose local endpoints to webhooks or external servers.
- **Postman / Insomnia / curl** to capture, craft, edit, and resend HTTP requests.
- **Charles Proxy / Fiddler / Wireshark** to inspect headers, timing, and JSON payloads.
- **Toxiproxy / Chaos Mesh** to simulate laggy networks, server crashes, and timeouts.
- **Custom scripts** to compare API responses when refactoring endpoints.

**Routa consolidates all of these into one unified local gateway.** With zero external dependencies, Routa lets you inspect, tunnel, route, mutate, mock, simulate failures, and diff traffic seamlessly on your machine.

---

## ⚡ Key Capabilities at a Glance

| Capability | What Routa Does |
| :--- | :--- |
| 🔍 **Live Traffic Inspector** | Real-time streaming web dashboard (`http://localhost:4040`) displaying full headers, JSON bodies, timing metrics, and status codes. |
| 🚀 **Local Tunneling & Relay** | Expose your local port (e.g. `3000`) securely to the public internet via a self-hosted edge Relay with WebSocket multiplexing. |
| 🔄 **1-Click Replay & Edit-Replay** | Re-fire captured requests instantly or edit HTTP methods, headers, parameters, and JSON payloads inline before resending. |
| 🔀 **Multi-Service Routing** | Route different URL paths (e.g. `/api/v1/auth/*`, `/api/v1/payments/*`) to different local ports and backend microservices. |
| 🛠️ **Traffic Mutation & Response Mocking** | Intercept traffic on-the-fly to inject/strip headers, rewrite URL paths, modify JSON fields via dot-paths (`user.role=admin`), or mock status responses. |
| 💣 **Network & Failure Simulator** | Test application resilience by injecting latency/jitter, simulating connection drops, forcing configurable error rates (e.g., 10% 500s), or enforcing timeouts. |
| 👥 **Shadow Traffic & Deep Response Differ** | Asynchronously duplicate live traffic to a secondary "shadow" URL (e.g., v2 API) and compare JSON responses side-by-side with color-coded diffing. |
| ⚓ **Webhook Testing Lab** | Auto-detect signature headers for providers (GitHub, Stripe, Shopify, Slack, Discord, Twilio, SendGrid, PayPal) and log delivery history. |
| 💾 **Session Fixtures & Playback** | Save request history into JSON session files and run deterministic playbacks with preserved inter-request timing. |

---

## 🏗️ Architecture Overview

```text
                        PUBLIC INTERNET / CLIENTS / WEBHOOKS
                                         |
                                         v
                              +--------------------+
                              |    Routa Relay     |  (Public Edge Server)
                              +--------------------+
                                         | WebSocket Tunnel
                                         v
                              +--------------------+
                              |    Routa Agent     |  (Local Gateway)
                              +--------------------+
                                 /       |       \
                                /        |        \
            +------------------+ +---------------+ +------------------+
            |  Dashboard UI    | |  Mutation &   | |   Webhook Lab    |
            |  (:4040 SPA)     | |  Simulator    | |   & Signature    |
            +------------------+ +---------------+ +------------------+
                                         |
                                         v
                              +--------------------+
                              |  Local Service(s)  |  (http://127.0.0.1:3000)
                              +--------------------+
```

---

## 🛠️ Everything You Can Do with Routa (With Clear Examples)

---

### 1. Expose a Local Backend to the Internet (Tunneling & Webhooks)

**The Use Case:** You are developing a local app on `http://localhost:3000` and need to test incoming webhooks from Stripe or GitHub, or demo your work to a teammate without deploying to staging.

**How to do it with Routa:**

Run `routa dev` with your local port:
```bash
# Basic inspection mode (Local only)
routa dev 3000

# Connect to a public Relay server (Generates a public URL)
routa dev 3000 --relay ws://relay.example.com:8080 --name my-app
```

**Output:**
```text
  Routa - Traffic Gateway

  Local target:  http://127.0.0.1:3000
  Dashboard:     http://localhost:4040

  Public URL:    http://my-app.relay.example.com:8080
  Subdomain:     my-app
```

*Now, any request sent to `http://my-app.relay.example.com:8080` is instantly multiplexed over WebSockets and proxied to your local `localhost:3000`.*

---

### 2. Inspect Real-Time HTTP Traffic in the Web Dashboard

**The Use Case:** Your frontend app gets an unexpected error when calling your API. You need to inspect exact raw headers, JSON payloads, timing breakdowns, and status codes.

**How to do it with Routa:**

1. Open **`http://localhost:4040`** in your browser while `routa dev` is running.
2. Requests stream in real-time over WebSocket push.
3. Click any request row to view:
   - **Headers Tab:** View incoming request headers and outgoing response headers.
   - **Body Tab:** View formatted/pretty-printed JSON payloads or raw text.
   - **Timing Tab:** See duration breakdown and proxy overhead.

---

### 3. Replay & Edit-Replay Captured Requests

**The Use Case:** You found a bug in a `POST /api/v1/orders` endpoint. Instead of using curl or Postman to recreate the complex JSON body, you want to modify a field and resend the request directly from your browser.

**How to do it with Routa:**

1. In the Dashboard (`http://localhost:4040`), select the target `POST` request.
2. Click **Replay** to resend the exact request immediately.
3. Or click **Edit & Replay** to open an interactive editor:
   - Change HTTP method (e.g. `POST` → `PUT`).
   - Add/edit headers (e.g. `Authorization: Bearer test-token`).
   - Edit the JSON request body (e.g. change `"quantity": 1` to `"quantity": 5`).
4. Click **Send Request** to fire the edited request. The new request will be tagged with a `Replay` badge in your stream.

---

### 4. Split Traffic Across Multiple Local Microservices (Declarative Routing)

**The Use Case:** You have multiple local services running on different ports (`auth` service on `8081`, `users` service on `8082`, `frontend` on `3000`) and want a single gateway entry point.

**How to do it with Routa:**

Define routing rules in `routa.yaml`:
```yaml
version: "1"
agent:
  port: 4040
  target: "http://localhost:3000"

routes:
  - path: "/api/v1/auth/*"
    target: "http://localhost:8081"
  - path: "/api/v1/users/*"
    target: "http://localhost:8082"
  - path: "/*"
    target: "http://localhost:3000"
```

Run Routa with the config:
```bash
routa dev --config routa.yaml
```

*Requests to `/api/v1/auth/login` seamlessly route to port `8081`, `/api/v1/users/profile` routes to `8082`, and all other traffic routes to `3000`.*

---

### 5. Mutate Request Headers, JSON Payloads & Mock Responses

**The Use Case:** 
- **Scenario A (Header/Body Injection):** Test how your application behaves when receiving an `admin` role without modifying your database or client code.
- **Scenario B (Response Mocking):** Work on a frontend feature against an unbuilt backend endpoint by mocking the API response.

**How to do it with Routa (`routa.yaml`):**

```yaml
mutations:
  # Scenario A: Header & JSON payload mutation
  - name: "Inject Admin Context"
    match:
      path: "/api/v1/profile"
      method: "POST"
    request:
      set_headers:
        X-Debug-Mode: "true"
        X-Tenant-ID: "tenant_123"
      remove_headers:
        - "X-Internal-Secret"
      add_query_params:
        trace: "enabled"
      set_body_json:
        "user.role": "admin"
        "user.permissions.can_delete": "true"
      remove_body_json:
        - "user.ssn"

  # Scenario B: Mock API response directly (Bypasses backend server)
  - name: "Mock Maintenance Status"
    match:
      path: "/api/v1/system-status"
    request:
      mock_response:
        status: 503
        body: '{"status": "maintenance", "message": "Scheduled upgrades in progress", "retry_in_seconds": 300}'
```

---

### 6. Inject Latency & Failures (Network Chaos Engineering)

**The Use Case:** You want to make sure your frontend app displays a proper loading spinner or retry prompt when the API suffers high latency, intermittent 500 errors, or sudden connection drops.

**How to do it with Routa (`routa.yaml`):**

```yaml
simulations:
  - name: "Staging Latency & Flaky Connection Test"
    match:
      path: "/api/v1/payments/*"
      method: "POST"
    latency_ms: 350       # Adds 350ms base delay
    jitter_ms: 75         # Latency varies by ±75ms (275ms - 425ms)
    error_rate: 0.15      # 15% of requests randomly fail
    error_status: 500     # Returns 500 Internal Server Error
    drop_rate: 0.02       # 2% of TCP connections drop abruptly
    timeout_ms: 2000      # Cancel requests taking longer than 2s
```

*You can also toggle and tweak these simulation settings dynamically live in the **Simulator** tab of the Web Dashboard!*

---

### 7. Shadow Traffic & Deep Response Differ (Safe API Migrations)

**The Use Case:** You rewritten your search service from Node.js (`localhost:3000`) to Go (`localhost:9090`). You want to duplicate live incoming requests to the new service and verify side-by-side that both services return identical JSON structures before swapping in production.

**How to do it with Routa (`routa.yaml`):**

```yaml
shadows:
  - name: "v2 Migration Search Test"
    match:
      path: "/api/v1/search"
    shadow_url: "http://localhost:9090"
    compare_response: true
```

**How it executes:**
1. Incoming request arrives at `/api/v1/search`.
2. Routa proxies the request to the primary service (`localhost:3000`) and returns its response to the user.
3. Simultaneously, Routa asynchronously clones the request and sends it to `localhost:9090`.
4. The **Deep Response Differ** engine compares HTTP status codes, headers, and nested JSON keys/values.
5. Visual side-by-side diff highlights show exact field mismatches in the Dashboard **Diff** tab.

---

### 8. Webhook Testing Lab & Provider Signature Verification

**The Use Case:** You are integrating third-party webhooks (e.g. Stripe checkout events or GitHub push events) and need to verify payload signatures and inspect incoming events locally.

**How to do it with Routa:**

1. Open the **Webhook Lab** tab in the Routa Dashboard.
2. Click **Create Endpoint** to generate a dedicated webhook URL.
3. Configure your third-party provider (Stripe, GitHub, Shopify, Slack, Discord, Twilio, SendGrid, PayPal) to deliver payloads to this URL.
4. Routa automatically detects provider signature headers (`X-Hub-Signature-256`, `Stripe-Signature`, etc.), verifies checksums, and formats the event history.

---

### 9. Session Persistence & Deterministic Playback

**The Use Case:** You captured a sequence of 20 API requests during a tricky bug reproduction session. You want to save this session fixture to share with teammates or run automated playback tests.

**How to do it with Routa:**

1. In the Web Dashboard, click **Save Session** and name it (e.g., `checkout-bug-repro`).
2. Routa saves the request collection as a JSON fixture under `~/.routa/sessions/checkout-bug-repro.json`.
3. To replay the sequence deterministically with preserved inter-request timing:
   - Click **Run Playback** in the Dashboard or trigger it via the API.
   - All recorded requests re-fire sequentially against your target backend.

---

## 📊 How Routa Compares to Other Tools

| Feature | Routa 🌐 | Ngrok | Postman | Fiddler / Charles | Toxiproxy |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Local HTTP Traffic Inspector** | ✅ | ⚠️ Basic | ❌ | ✅ | ❌ |
| **Public WebSocket Tunneling** | ✅ | ✅ | ❌ | ❌ | ❌ |
| **1-Click & Edit-Replay** | ✅ | ❌ | ✅ | ⚠️ Partial | ❌ |
| **Multi-Service Microservice Routing** | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Header & JSON Body Mutation** | ✅ | ❌ | ❌ | ⚠️ Scripted | ❌ |
| **Network Chaos Simulation** | ✅ | ❌ | ❌ | ❌ | ✅ |
| **Shadow Traffic & JSON Differ** | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Webhook Lab & Provider Signatures** | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Single Standalone Binary (Go)** | ✅ | ✅ | ❌ (Heavy GUI) | ❌ | ✅ |

---

## 🚀 Quick CLI Reference

```bash
# 1. Start dev traffic inspector & dashboard for local port 3000
routa dev 3000

# 2. Expose local app on a self-hosted Relay server
routa dev 3000 --relay ws://relay.example.com:8080 --name my-service

# 3. Launch with YAML configuration file
routa dev --config routa.yaml

# 4. Run a public Relay server (on edge/cloud instance)
routa relay --port 8080 --domain relay.example.com:8080

# 5. Check version
routa version
```

---

## 📄 License

Routa is open-source software licensed under the [MIT License](LICENSE).
