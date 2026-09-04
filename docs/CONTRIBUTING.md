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
   git clone https://github.com/YOUR-USERNAME/routa.git
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
├── cli/         # Command-line parser, banner, & terminal formatting
├── cmd/routa/   # Entry point (main.go)
├── config/      # Configuration structs, env vars, & YAML loader
├── diff/        # HTTP response comparator & deep JSON body differ
├── middleware/  # Traffic mutation & fault simulation middleware
├── protocol/    # Wire format binary framing & JSON payload messaging
├── proxy/       # Reverse HTTP proxy & response capture engine
├── recorder/    # Thread-safe ring buffer for request history
├── relay/       # Edge Relay server & client registry
├── replay/      # Request replay & edit-replay engine
├── router/      # Pattern-based HTTP router
├── shadow/      # Shadow traffic forwarder & comparison pipeline
├── storage/     # Session persistence & deterministic playback runner
├── tunnel/      # Persistent WebSocket tunnel client
└── webhook/     # Webhook lab, provider detector, & delivery history
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

### Test Binary Compilation

```bash
go build ./cmd/routa/...
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
