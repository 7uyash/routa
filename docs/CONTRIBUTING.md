# 🤝 Contributing to Routa

Thank you for your interest in contributing to **Routa**! We welcome contributions of all kinds—bug fixes, new features, documentation improvements, performance optimizations, and design tweaks.

---

## 📋 Code of Conduct

We aim to foster an open, welcoming, and inclusive community. Please be respectful and constructive in all interactions.

---

## 🛠️ Development Setup

### Prerequisites

- **Go**: Version 1.21 or higher installed (`go version`).
- **Git**: For version control.

### Getting the Code

1. Fork the repository on GitHub.
2. Clone your fork locally:
   ```bash
   git clone https://github.com/7uyash/routa.git
   cd routa
   ```
3. Add the upstream repository:
   ```bash
   git remote add upstream https://github.com/ORIGINAL-OWNER/routa.git
   ```

---

## 🏗️ Codebase Architecture

Routa is organized into clean, modular Go packages with minimal external dependencies.

```text
Routa
├── agent/       # Local agent daemon & embedded Web Dashboard (REST API + WS)
├── cli/         # Command-line parser, interactive TUI, and banner
├── cmd/routa/   # Entry point (main.go)
├── config/      # Configuration structs, env vars, YAML loader & project config
├── diff/        # HTTP response comparator & deep JSON body differ
├── discovery/   # Local port scanner & tech-stack inference
├── middleware/  # Traffic mutation & fault simulation middleware
├── mock/        # Mock lab — fixed-response endpoint definitions
├── protocol/    # Wire format binary framing & JSON payload messaging
├── proxy/       # Reverse HTTP proxy with timing capture
├── recorder/    # Thread-safe ring buffer for request history
├── relay/       # Edge relay server & client registry
├── replay/      # Request replay & edit-replay engine
├── router/      # Pattern-based HTTP router (supports multiple local targets)
├── shadow/      # Shadow traffic forwarder & comparison pipeline
├── simulator/   # (reserved for future standalone simulator)
├── storage/     # Session persistence & scenario/playback runner
├── traffic/     # Canonical Request, Response & Timing models
├── tunnel/      # Persistent WebSocket tunnel client with reconnect
└── webhook/     # Webhook lab, provider detector & delivery history
```

---

## 🧪 Testing & Verification

Before submitting any code changes, ensure all tests pass and static checks are clean.

### Run All Unit Tests

```bash
go test -v ./...
```

### Run Static Analysis (`go vet`)

```bash
go vet ./...
```

### Test Binary Compilation & Multi-Platform Cross-Building

Verify that local compilation succeeds, or test cross-platform builds:

```bash
# Local machine compilation
go build -o bin/routa ./cmd/routa

# Cross-compile all targets (macOS ARM64/AMD64, Linux ARM64/AMD64, Windows ARM64/AMD64)
# On Linux/macOS:
make build-all

# On Windows (PowerShell):
.\build.ps1
```

---

## 📐 Coding Guidelines & Standards

1. **Idiomatic Go**: Follow standard Go formatting (`gofmt`) and naming conventions (`camelCase` for private, `PascalCase` for exported).
2. **Thread Safety**: Routa handles high-concurrency network traffic. Always ensure shared state (e.g., in memory buffers, maps, registries) is properly guarded using standard Go mutexes (`sync.RWMutex`, `sync.Mutex`) or atomic operations.
3. **Zero / Low Dependencies**: Keep external dependencies minimal to preserve fast build times and binary portability.
4. **Documentation & Comments**: Add godoc comments to exported functions, types, and constants. Preserve docstrings and comments on untouched files.
5. **No Panic in Production Code**: Handle errors explicitly using Go error returns (`if err != nil`). Avoid standard library `panic()` calls in core packages.

---

## 🔀 Pull Request Process

1. **Create a Feature Branch**:
   ```bash
   git checkout -b feature/my-cool-feature
   ```
2. **Make your changes** and commit with descriptive messages:
   ```bash
   git commit -m "feat(middleware): add query string mutation support"
   ```
3. **Rebase against upstream `main`**:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```
4. **Push to your fork** and open a Pull Request against `main`.
5. Clearly describe the motivation, summary of changes, and how you verified your changes in your PR description.

---

## 💬 Getting Help

If you have questions or need assistance, feel free to open an issue or start a discussion on the GitHub repository.

Happy coding! 🚀
