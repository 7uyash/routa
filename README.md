<div align="center">

  <p><img src="agent/dashboard/static/logo.png" alt="Routa logo" width="360" /></p>

  <h1>Routa</h1>

  <p><strong>The developer traffic gateway for local HTTP services</strong></p>

  <p>
    <a href="https://github.com/7uyash/routa/stargazers"><img src="https://img.shields.io/github/stars/7uyash/routa" alt="GitHub stars" /></a>
    <a href="https://github.com/7uyash/routa/graphs/contributors"><img src="https://img.shields.io/github/contributors/7uyash/routa" alt="Contributors" /></a>
    <a href="go.mod"><img src="https://img.shields.io/badge/Go-1.26.4%2B-00ADD8?logo=go&amp;logoColor=white" alt="Go 1.26.4 or newer" /></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="MIT license" /></a>
  </p>

  <p>Inspect, replay, mock, and simulate HTTP traffic through a local proxy and live dashboard. Connect an optional self-hosted relay to expose a local service remotely.</p>

</div>

## Quick start

Routa requires **Go 1.26.4 or newer** (see [`go.mod`](go.mod)). To use the code in this repository, clone it and install from the checkout:

```bash
git clone https://github.com/7uyash/routa.git
cd routa
go install ./cmd/routa
routa dev 3000
```

Replace `3000` with the port of your running HTTP service. Open the dashboard at <http://localhost:4040>, then send requests to <http://localhost:4000>. For example, visit `http://localhost:4000/` to reach your app's `/` route through Routa. Requests sent directly to the app's port bypass Routa and do not appear in its inspector.

If you do not know the target port yet, run `routa dev` and select a service or enter its URL in the dashboard. The dashboard also opens automatically when `dev` starts.

> **Installing from Go releases:** `go install github.com/7uyash/routa/cmd/routa@latest` selects the latest tagged module release. The `v0.1.0` release predates the separate port `4000` proxy. If `@latest` installs that release, use `go install ./cmd/routa` from a checkout containing the feature. To inspect the installed build, run `go version -m "$(go env GOPATH)/bin/routa"` (use `routa.exe` on Windows).

## How local mode works

```text
Browser or API client  →  localhost:4000 (local HTTP proxy)
                              ↓ inspect, mutate, simulate, record
                         localhost:3000 (your service)

Dashboard             →  localhost:4040
```

The dashboard lets you inspect requests and responses, replay requests, choose a target, configure routes and mutation rules, create mock responses, simulate delays and errors, discover local services, compare shadow responses, and record scenarios. The proxy handles HTTP requests; it does not implement WebSocket upgrade forwarding. Both listeners use their configured ports (`4040` and `4000` by default). Check the terminal for a port binding error if either URL does not open.

The green `localhost:4000` URL in the dashboard is the **local proxy**, not a public relay address. The **Local Only** status means no relay URL was configured. The top **Requests** count currently tracks tunnel requests; locally proxied requests can appear in the inspector while that count remains zero.

### Dev command

```text
routa dev [target-port] [flags]
```

| Flag | Default | Purpose |
| --- | --- | --- |
| `--dashboard <port>` | `4040` | Dashboard HTTP port |
| `--proxy <port>` | `4000` | Local HTTP proxy port |
| `--host <host>` | `localhost` | Host of the local target |
| `--max-entries <n>` | `500` | Maximum requests kept in the inspector |
| `--relay <url>` | none | WebSocket address of a self-hosted relay |
| `--name <name>` | generated | Requested public subdomain when using a relay |
| `--token <token>` | none | Token sent to the relay; the current relay does not validate it |
| `--auth-user`, `--auth-pass` | none | Accepted by the CLI; public endpoint basic authentication is not wired into the relay |

For example, `routa dev 3000 --proxy 4100 --dashboard 4140` uses `localhost:4100` for the app proxy and `localhost:4140` for the dashboard. Put flags after the optional target port.

## Optional public relay

Run the relay on a server reachable from the internet, with wildcard DNS for the subdomains you intend to use. For an HTTPS public URL, terminate TLS at a reverse proxy and forward HTTP and WebSocket traffic to Routa's relay port:

```bash
# On the public server, behind a TLS reverse proxy:
routa relay --port 8080 --domain example.com

# On the machine running your local app on port 3000:
routa dev 3000 --relay wss://example.com --name my-app
```

With suitable DNS and TLS setup, the relay returns `https://my-app.example.com`. The relay itself listens over plain HTTP on its configured port; it does not set up TLS or DNS. The current relay accepts the agent's token without checking it, and the CLI's basic auth flags do not protect public traffic. Do not expose sensitive services through it without access controls at your reverse proxy.

The `relay` command also accepts `--host <host>` (default `0.0.0.0`) and `--domain <domain>` (default `localhost`). Running `routa relay` without flags opens an interactive setup form.

## Configuration and storage

CLI flags and environment variables configure the running agent. Supported environment variables include `ROUTA_LOCAL_PORT`, `ROUTA_RELAY_URL`, `ROUTA_AUTH_TOKEN`, `ROUTA_DASHBOARD_PORT`, `ROUTA_PROXY_PORT`, `ROUTA_TUNNEL_NAME`, `ROUTA_BASIC_AUTH_USER`, `ROUTA_BASIC_AUTH_PASS`, `ROUTA_BASE_DOMAIN`, `ROUTA_RELAY_PORT`, and `ROUTA_DATA_DIR`.

There is a parser for `routa.yaml` in the source, but the CLI **does not call it yet**. Placing that file in a project directory currently has no effect. Configure rules through the dashboard instead.

Saved sessions and scenarios are written under `~/.routa/sessions/` by default. Set `ROUTA_DATA_DIR` to use another base directory.

## Build and test

```bash
go build -o routa ./cmd/routa
go test ./...
go vet ./...
```

At present, `go vet ./...` reports a lock-copy warning in `storage/scenario_engine.go`; this is an existing source issue, independent of the README.

For cross-platform binaries, run `make build-all` on Linux/macOS or `./build.ps1` in PowerShell on Windows.

## Star history

[![Star history for 7uyash/routa](https://api.star-history.com/svg?repos=7uyash/routa&type=date)](https://www.star-history.com/?repos=7uyash%2Frouta&type=date)

## License

[MIT](LICENSE)
