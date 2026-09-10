# 🌐 About Routa

**Routa** (pronounced */ru-ta/*) is an all-in-one **High-Performance Developer Traffic Gateway, Local Tunneling System, HTTP Inspector, and Chaos Engineering Toolkit** written in Go.

It sits as an intelligent layer between incoming network traffic (frontend applications, public webhooks, remote clients) and your local backend microservices. Routa gives developers **100% real-time visibility, total traffic control, automatic API discovery, zero-config service discovery, an instant mock lab, and advanced simulation capabilities** over every single HTTP request and response — all through a single, lightweight binary and an embedded web dashboard.

---

## 🎯 What Problem Does Routa Solve?

During backend and API development, developers often have to stitch together multiple separate tools:
- **Ngrok / Localtunnel** to expose local endpoints to webhooks or external servers.
- **Postman / Insomnia / curl** to capture, craft, edit, and resend HTTP requests.
- **Charles Proxy / Fiddler / Wireshark** to inspect headers, timing, and JSON payloads.
- **Mockoon / Prism / WireMock** to manually stub out endpoints and create mock servers.
- **Toxiproxy / Chaos Mesh** to simulate laggy networks, server crashes, and timeouts.
- **Custom scripts** to compare API responses when refactoring endpoints.

**Routa consolidates all of these into one unified local gateway.** Combined with Zero-Config Discovery, Automatic API Mapping, the 1-Click Mock Lab, the "Connect Anything" Public Webhook Gateway, and **native cross-platform support for ARM64 and AMD64 (macOS Apple Silicon, Linux, and Windows)**, Routa becomes the complete, end-to-end traffic control center for local development.

---

## 💻 Cross-Platform & ARM64 Binary Support

Routa is compiled for native performance across all major operating systems and CPU architectures:

- **macOS Apple Silicon (ARM64)**: Native binary optimized for Apple M1, M2, M3, M4 Macs (`bin/routa-darwin-arm64`).
- **macOS Intel (AMD64)**: Native binary for x86_64 Mac computers (`bin/routa-darwin-amd64`).
- **Linux (ARM64 & AMD64)**: Native binaries for Ubuntu, Debian, Fedora, Arch, Raspberry Pi, and ARM servers (`bin/routa-linux-arm64` & `bin/routa-linux-amd64`).
- **Windows (ARM64 & AMD64)**: Native `.exe` executables for ARM64 Windows laptops and standard x64 Windows PCs (`bin/routa-windows-arm64.exe` & `bin/routa-windows-amd64.exe`).

### 📦 Complete OS-by-OS Installation & Execution Guide

#### 🍏 1. macOS (Apple Silicon M1/M2/M3/M4 & Intel)

##### Step 1: Prepare the Binary
```bash
# For Apple Silicon (M1/M2/M3/M4):
chmod +x bin/routa-darwin-arm64

# For Intel Macs:
chmod +x bin/routa-darwin-amd64
```

##### Step 2: Install to System PATH
```bash
# Apple Silicon:
sudo mv bin/routa-darwin-arm64 /usr/local/bin/routa

# Intel Mac:
sudo mv bin/routa-darwin-amd64 /usr/local/bin/routa
```

##### Step 3: Execute & Run
```bash
# Start local gateway forwarding to port 3000
routa dev 3000

# Open Web Inspector Dashboard in browser:
# http://localhost:4040
```

---

#### 🐧 2. Linux (Ubuntu, Debian, Fedora, Arch, Raspberry Pi)

##### Step 1: Prepare the Binary
```bash
# For Linux ARM64:
chmod +x bin/routa-linux-arm64

# For Linux AMD64 (x86_64):
chmod +x bin/routa-linux-amd64
```

##### Step 2: Install to System PATH
```bash
# Linux ARM64:
sudo mv bin/routa-linux-arm64 /usr/local/bin/routa

# Linux AMD64:
sudo mv bin/routa-linux-amd64 /usr/local/bin/routa
```

##### Step 3: Execute & Run
```bash
# Run Routa on port 3000
routa dev 3000
```

---

#### 🪟 3. Windows (Command Prompt / PowerShell)

##### Option A: Using Pre-Built Executable
```powershell
# For 64-bit Windows PC (AMD64):
.\bin\routa-windows-amd64.exe dev 3000

# For ARM64 Windows PC:
.\bin\routa-windows-arm64.exe dev 3000
```

##### Option B: Install Globally in PowerShell
```powershell
Copy-Item .\bin\routa-windows-amd64.exe C:\Windows\System32\routa.exe
routa dev 3000
```

---

#### 🛠️ 4. Build from Source (Any OS)

If you have Go installed (`go 1.21+`):

```bash
# Build binary for your local OS
go build -o routa ./cmd/routa

# Cross-compile for ALL operating systems & architectures at once:
# On Linux/macOS:
make build-all

# On Windows (PowerShell):
.\build.ps1
```

---

## ⚡ Key Capabilities at a Glance

| Capability | What Routa Does |
| :--- | :--- |
| 🔍 **Live Traffic Inspector** | Real-time streaming web dashboard (`http://localhost:4040`) displaying full headers, JSON bodies, timing metrics, and status codes. |
| 🕵️ **Zero-Config Service Discovery** | Automatically scans local ports, detects running HTTP services, suggests friendly names, and proposes route rules with user confirmation. |
| 🗺️ **Automatic API Discovery & Mapping** | Watches live traffic, groups endpoints into a visual map, normalizes dynamic paths (`/users/123` → `/users/{id}`), and tracks per-endpoint metrics. |
| 🧪 **Mock Lab (1-Click Traffic-to-Mock)** | Instantly convert captured traffic into local mock endpoints. Click "Create Mock", tweak response status/body, simulate delays, and serve. |
| 🌐 **"Connect Anything" Webhook Gateway** | Create public webhook endpoints for Stripe, GitHub, or any custom service with signature validation, ON/OFF toggles, and test connection buttons. |
| 🚀 **Local Tunneling & Relay** | Expose your local port (e.g. `3000`) securely to the public internet via a self-hosted edge Relay with WebSocket multiplexing. |
| 🔄 **1-Click Replay & Edit-Replay** | Re-fire captured requests instantly or edit HTTP methods, headers, parameters, and JSON payloads inline before resending. |
| 🔀 **Multi-Service Routing** | Route different URL paths (e.g. `/api/v1/auth/*`, `/api/v1/payments/*`) to different local ports and backend microservices. |
| 🛠️ **Traffic Mutation & Response Mocking** | Intercept traffic on-the-fly to inject/strip headers, rewrite URL paths, modify JSON fields via dot-paths (`user.role=admin`), or mock status responses. |
| 💣 **Network & Failure Simulator** | Test application resilience by injecting latency/jitter, simulating connection drops, forcing configurable error rates (e.g., 10% 500s), or enforcing timeouts. |
| 👥 **Shadow Traffic & Deep Response Differ** | Asynchronously duplicate live traffic to a secondary "shadow" URL (e.g., v2 API) and compare JSON responses side-by-side with color-coded diffing. |
| 💾 **Session Fixtures & Playback** | Save request history into JSON session files and run deterministic playbacks with preserved inter-request timing. |

---

## 🏗️ Architecture Overview

```text
                        PUBLIC INTERNET / CLIENTS / WEBHOOKS / CONNECT ANYTHING
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
            |  Dashboard UI    | |  Service &    | |  Mock Lab &      |
            |  (:4040 SPA)     | |  API Discovery| |  Connect Anything|
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

### 1. Zero-Config Local Service Discovery

**The Use Case:** You have 4 backend microservices running across different local ports (`3000`, `8080`, `8081`, `5000`) and don't want to manually locate ports or type out complex proxy configs.

**How it works in Routa:**

1. Routa automatically scans local ports upon startup.
2. The Dashboard displays an active list of detected HTTP services with auto-suggested friendly names (e.g. `Node.js App (:3000)`, `Go Auth Service (:8081)`).
3. **1-Click Action:** Click on any detected service to route traffic to it, inspect its requests, or open it in your browser.
4. **Safety-First Routing Proposals:** Routa detects unmapped traffic patterns and proposes new routing rules in the dashboard for **user confirmation** — preventing accidental breaking changes to your environment.

---

### 2. Automatic API Discovery & Visual Endpoint Mapping

**The Use Case:** As your application runs, you want to see a clean, organized map of your entire API surface with per-endpoint latency, request counts, and error rates, without writing OpenAPI/Swagger specs manually.

**How it works in Routa:**

1. Routa observes live incoming and outgoing HTTP traffic in real time.
2. It intelligently normalizes parameter paths (e.g., automatically collapsing `/users/123` and `/users/456` into `/users/{id}`).
3. Endpoints are grouped visually in an **API Map** displaying:
   - Request volumes & HTTP method distribution (`GET`, `POST`, `PUT`, `DELETE`).
   - Average latency and error rates per normalized route.
   - Request and response body schema previews.

---

### 3. The Instant Mock Lab (Traffic-to-Mock in 1-Click)

**The Use Case:** Your frontend team needs to work on a new feature, but the backend API endpoint isn't finished yet — or you need to reproduce a rare 500 error response without altering server code.

**How to do it with Routa:**

- **Easiest Flow (1-Click Traffic-to-Mock):**
  1. Find any real captured request in your Dashboard request stream.
  2. Click **"Create Mock"**.
  3. Routa auto-populates the HTTP method, normalized URL path, headers, and captured JSON response body.
  4. Tweak the status code (e.g., `200 OK` or `500 Internal Error`), edit the JSON payload if needed, and set an optional delay (e.g., `250ms`).
  5. Click **Save & Activate**. Routa instantly serves this mock locally, bypassing the backend!

- **Manual Mock Creation:**
  Define custom mocks directly in the Dashboard **Mock Lab** or in `routa.yaml`:
  ```yaml
  mutations:
    - name: "Mock Order Status API"
      match:
        path: "/api/v1/orders/status"
      request:
        mock_response:
          status: 200
          body: '{"order_id": "ord_999", "status": "processing", "mocked": true}'
  ```

---

### 4. "Connect Anything" — Universal Webhook & Public Gateway

**The Use Case:** You need to integrate webhooks from Stripe, GitHub, Shopify, Twilio, Slack, or any custom third-party provider into your local development machine — without setting up complex public servers or exposing raw ports.

**How it works in Routa:**

1. Open the **Connect Anything** tab in the Routa Dashboard.
2. Choose a pre-configured provider template (**Stripe**, **GitHub**, **Shopify**, **Slack**, **Discord**) or select **Custom Webhook** to **connect anything**.
3. Routa generates a secure public URL (e.g. `http://my-app.relay.example.com:8080/webhook/wh_xyz123`).
4. **Key Features & Controls:**
   - **Signature Verification:** Built-in secret key verification options (`X-Hub-Signature-256`, `Stripe-Signature`, etc.).
   - **Connection ON/OFF Toggle:** Disable incoming webhooks with a single toggle switch without deleting the endpoint configuration.
   - **Test Connection Simulator:** Click **"Test Connection"** to fire mock test payloads directly to your local handler to verify your logic *before* real external traffic hits.
   - **Full Event History:** Reuses Routa's core traffic inspection, timing breakdown, and 1-click request replay pipelines.

---

### 5. Expose a Local Backend to the Internet (Tunneling & Relay)

**The Use Case:** You want to expose your local web application on `http://localhost:3000` to external users or remote team members.

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

---

### 6. Inspect Real-Time HTTP Traffic in the Web Dashboard

**The Use Case:** Debugging why a client application receives a `400 Bad Request` or inspecting raw request/response headers and body frames.

**How to do it with Routa:**

1. Open **`http://localhost:4040`** in your browser while `routa dev` is running.
2. Requests stream in real-time over WebSocket push with zero page refreshes.
3. Inspect headers, pretty-printed JSON payloads, query parameters, and proxy processing time.

---

### 7. Replay & Edit-Replay Captured Requests

**The Use Case:** Fixing a bug in a `POST /api/v1/orders` endpoint without using curl or Postman to recreate complex JSON payloads.

**How to do it with Routa:**

1. Select the captured request in the Dashboard (`http://localhost:4040`).
2. Click **Replay** to resend the exact request immediately.
3. Click **Edit & Replay** to open the interactive editor:
   - Change HTTP method (`POST` → `PUT`).
   - Add/edit headers (`Authorization: Bearer test-token`).
   - Modify JSON request body fields inline.
4. Click **Send Request** to execute. Replayed requests are tagged with a `Replay` badge.

---

### 8. Multi-Service Microservice Routing

**The Use Case:** Split traffic across multiple local backend microservices using declarative route rules in `routa.yaml`:

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

---

### 9. Mutate Request Headers & JSON Payloads

**The Use Case:** Test authorization roles or strip sensitive tokens on-the-fly without altering client code:

```yaml
mutations:
  - name: "Inject Admin Context"
    match:
      path: "/api/v1/profile"
      method: "POST"
    request:
      set_headers:
        X-Debug-Mode: "true"
      set_body_json:
        "user.role": "admin"
      remove_body_json:
        - "user.ssn"
```

---

### 10. Inject Latency & Failures (Network Chaos Engineering)

**The Use Case:** Test frontend loading spinners, timeouts, and error handling against unpredictable network conditions:

```yaml
simulations:
  - name: "Staging Latency & Flaky Connection Test"
    match:
      path: "/api/v1/payments/*"
    latency_ms: 350       # Adds 350ms base delay
    jitter_ms: 75         # Latency varies ±75ms
    error_rate: 0.15      # 15% random 500 errors
    error_status: 500
    drop_rate: 0.02       # 2% connection drops
```

---

### 11. Shadow Traffic & Deep Response Differ

**The Use Case:** Asynchronously duplicate live traffic to a secondary target (`localhost:9090`) and run deep JSON diffing to verify API refactors before swapping in production:

```yaml
shadows:
  - name: "v2 Migration Search Test"
    match:
      path: "/api/v1/search"
    shadow_url: "http://localhost:9090"
    compare_response: true
```

---

### 12. Session Persistence & Deterministic Playback

**The Use Case:** Save request streams into JSON session fixtures (`~/.routa/sessions/bug-repro.json`) and replay them sequentially with realistic timing intervals for automated regression testing.

---

## 📊 How Routa Compares to Other Tools

| Feature | Routa 🌐 | Ngrok | Postman | Prism / Mockoon | Toxiproxy |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Zero-Config Service Discovery** | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Automatic API Discovery & Mapping** | ✅ | ❌ | ❌ | ❌ | ❌ |
| **1-Click Traffic-to-Mock Lab** | ✅ | ❌ | ⚠️ Manual | ✅ | ❌ |
| **"Connect Anything" Webhook Gateway** | ✅ | ⚠️ Basic | ❌ | ❌ | ❌ |
| **Local HTTP Traffic Inspector** | ✅ | ⚠️ Basic | ❌ | ❌ | ❌ |
| **Public WebSocket Tunneling** | ✅ | ✅ | ❌ | ❌ | ❌ |
| **1-Click & Edit-Replay** | ✅ | ❌ | ✅ | ❌ | ❌ |
| **Multi-Service Microservice Routing** | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Header & JSON Body Mutation** | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Network Chaos Simulation** | ✅ | ❌ | ❌ | ❌ | ✅ |
| **Shadow Traffic & JSON Differ** | ✅ | ❌ | ❌ | ❌ | ❌ |
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
