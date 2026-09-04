# ⚡ Routa

> **High-Performance Developer Traffic Gateway, Local Tunneling, & Inspection Platform in Go.**

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)]()

**Routa** is a lightweight, all-in-one developer traffic proxy, HTTP inspector, public tunnel gateway, and chaos engineering toolkit. It gives backend developers, API engineers, and frontend teams instant visibility and total control over inbound and outbound HTTP traffic—without needing complex cloud proxy setups.

---

## ✨ Features

- 🚀 **Local HTTP Tunneling & Relay** — Expose local HTTP services to the internet via a self-hosted Relay server with automatic WebSocket connection management, multiplexing, heartbeat monitoring, and automatic reconnection.
- 🔍 **Live Traffic Inspector** — Embedded web dashboard (`http://localhost:4040`) featuring real-time WebSocket push updates, full request/response headers & body inspection, timing breakdowns, and search/filtering.
- 🔁 **Replay & Edit-Replay** — Re-fire any recorded HTTP request with a single click or modify headers, query params, and JSON request bodies inline before re-sending.
- 🔀 **Multi-Service Routing** — Declarative pattern matching (`/api/v1/*`, `/auth/*`, `/*`) to route traffic to multiple local backend microservices seamlessly.
- 🎣 **Webhook Testing Lab** — Inspect incoming webhooks instantly with provider auto-detection (GitHub, Stripe, Shopify, Discord, Slack, Twilio, SendGrid, PayPal) and signature header verification.
- 🛠️ **Traffic Mutation & Mock Engine** — Inject/strip headers, rewrite URL paths, modify JSON request bodies via dot-path expressions, mock JSON responses, or force HTTP status codes on the fly.
- ⚡ **Network & Failure Simulator** — Test application resilience by injecting fixed/jittered latency, simulating connection drops, forcing configurable error rates (e.g. 50% 500 errors), or imposing artificial backend timeouts.
- 👥 **Shadow Traffic & Deep Response Differ** — Forward production/staging traffic asynchronously to a shadow target URL and compare responses side-by-side with deep JSON body diffing.
- 💾 **Session Persistence & Playback** — Export request collections into session fixtures and run deterministic sequential playbacks with preserved inter-request timing.

---

## 🏗️ Architecture Overview

```
                        PUBLIC INTERNET
                               │
                       ┌───────▼───────┐
                       │  Routa Relay  │ (Public Edge Server)
                       └───────┬───────┘
                               │ WebSocket Tunnel (Multiplexed Frames)
                               │
                       ┌───────▼───────┐
                       │  Routa Agent  │ (Local Machine)
                       └───────┬───────┘
             ┌─────────────────┼─────────────────┐
             │                 │                 │
     ┌───────▼───────┐ ┌───────▼───────┐ ┌───────▼───────┐
     │ Dashboard UI  │ │ Mutation /    │ │ Webhook Lab   │
     │  (:4040 SPA)  │ │ Simulator Pipeline             │
     └───────────────┘ └───────┬───────┘ └───────────────┘
                               │
                       ┌───────▼───────┐
                       │ Local Service │ (http://127.0.0.1:3000)
                       └───────────────┘
```

---

## 🚀 Quick Start

### Prerequisites

- [Go 1.21+](https://go.dev/dl/) installed.

### Installation

Clone the repository and build the binary:

```bash
git clone https://github.com/7uyash/routa.git
cd routa
go build -o routa ./cmd/routa
```

---

## 💡 Usage

### 1. Dev Mode (Tunnel Local Service)

Forward traffic from a remote Relay to a local HTTP service running on port `3000`:

```bash
# Basic usage
routa dev 3000

# Connect to a custom self-hosted Relay server
routa dev 3000 --relay ws://relay.example.com:8080 --name my-app
```

**Output:**
```text
  Routa — Traffic Gateway

  Local target:  http://127.0.0.1:3000
  Dashboard:     http://localhost:4040

  Public URL:    http://my-app.relay.example.com:8080
  Subdomain:     my-app
```

Open `http://localhost:4040` in your browser to launch the **Routa Dashboard**.

---

### 2. Relay Mode (Self-Hosted Edge Server)

Run a public relay edge server to terminate incoming web requests and tunnel them to connected agents:

```bash
routa relay --port 8080 --domain relay.example.com:8080 --secret your-optional-auth-token
```

---

### 3. Declarative Config (`routa.yaml`)

Define multi-service routes, mutation rules, fault injection, and shadow targets declaratively:

```yaml
version: "1"
agent:
  port: 4040
  target: "http://localhost:3000"

routes:
  - path: "/api/v1/users/*"
    target: "http://localhost:8081"
  - path: "/api/v1/payments/*"
    target: "http://localhost:8082"

mutations:
  - name: "Inject Debug Header"
    match:
      path: "/api/*"
    request:
      set_headers:
        X-Debug-Mode: "true"
      remove_headers:
        - "X-Internal-Token"

simulations:
  - name: "Staging Latency & Error Test"
    match:
      path: "/api/v1/payments/*"
    latency_ms: 250
    jitter_ms: 50
    error_rate: 0.1
    error_status: 503

shadows:
  - name: "Canary Testing"
    match:
      path: "/api/v1/search"
    shadow_url: "http://localhost:9090"
    compare_response: true
```

Run with configuration:

```bash
routa dev --config routa.yaml
```

---

## 📚 Documentation

Detailed documentation is available in the [`docs/`](./docs) directory:

- 📖 [**User Guide**](./docs/USAGE.md) — Comprehensive guide on CLI commands, dashboard features, webhook lab, mutation rules, simulations, and playback.
- ⚙️ [**Configuration Reference**](./docs/CONFIGURATION.md) — Complete `routa.yaml` schema documentation.
- 🤝 [**Contributing Guide**](./docs/CONTRIBUTING.md) — Architecture breakdown, package layout, dev setup, and pull request guidelines.

---

## 🛠️ Project Structure

| Directory | Description |
|-----------|-------------|
| [`agent/`](./agent) | Local agent daemon, embedded Web Dashboard, REST API & WebSocket handlers |
| [`cli/`](./cli) | CLI argument parsing, flags, terminal UI, and banner rendering |
| [`cmd/routa/`](./cmd/routa) | Application entry point (`main.go`) |
| [`config/`](./config) | Configuration models, environment variable binding, & YAML parser |
| [`diff/`](./diff) | HTTP response comparator & deep JSON body differ |
| [`middleware/`](./middleware) | Traffic mutation, mock response, & fault simulation middleware |
| [`protocol/`](./protocol) | Binary wire format framing & JSON message payloads |
| [`proxy/`](./proxy) | Forwarding HTTP proxy engine with timing & header normalization |
| [`recorder/`](./recorder) | High-performance in-memory ring buffer for traffic history |
| [`relay/`](./relay) | Edge Relay server & agent connection registry |
| [`replay/`](./replay) | Request replay and edit-replay execution engine |
| [`router/`](./router) | Pattern-based HTTP request routing engine |
| [`shadow/`](./shadow) | Shadow traffic forwarder & dual-target execution |
| [`storage/`](./storage) | Session persistence (JSON fixtures) & deterministic playback runner |
| [`tunnel/`](./tunnel) | Persistent WebSocket tunnel client with reconnect & ping/pong |
| [`webhook/`](./webhook) | Webhook lab, provider detector, & delivery history tracker |

---

## 🧪 Testing

Run all unit tests across packages:

```bash
go test -v ./...
```

Run static analysis:

```bash
go vet ./...
```

---

## 📄 License

Routa is open-source software licensed under the [MIT License](LICENSE).
