# claw2claw 🦀↔️🦀

Secure peer-to-peer context sharing for Claude plugins. Share markdown files, notes, and context between Claude instances with end-to-end encryption.

## Overview

claw2claw enables two Claude users to securely share context without any intermediary (including the relay server) seeing the content. Built on PAKE (Password-Authenticated Key Exchange) for cryptographic security.

## Architecture

```
┌─────────────────┐                      ┌─────────────────┐
│   Claude A      │                      │   Claude B      │
│   (Sender)      │                      │   (Receiver)    │
└────────┬────────┘                      └────────┬────────┘
         │                                        │
         │  claw2claw plugin                      │  claw2claw plugin
         │                                        │
┌────────▼────────┐                      ┌────────▼────────┐
│  Local Client   │                      │  Local Client   │
│  - PAKE keygen  │                      │  - PAKE keygen  │
│  - E2E encrypt  │                      │  - E2E decrypt  │
└────────┬────────┘                      └────────┬────────┘
         │                                        │
         │         ┌──────────────────┐           │
         │         │  claw2claw-app   │           │
         └────────►│     (Relay)      │◄──────────┘
                   │                  │
                   │  • Room mgmt     │
                   │  • No content    │
                   │    visibility    │
                   │  • Connection    │
                   │    brokering     │
                   └──────────────────┘
```

## Security Model

1. **PAKE Key Exchange**: Both parties derive identical encryption keys from a shared code phrase without transmitting the key
2. **End-to-End Encryption**: Content is encrypted before leaving the client; relay never sees plaintext
3. **Zero-Knowledge Relay**: The relay server only sees encrypted blobs and connection metadata
4. **Perfect Forward Secrecy**: Each session generates new keys

## Features

- 📝 Share .md files and context notes
- 🔐 End-to-end encryption (AES-256-GCM)
- 🔑 PAKE-based key exchange (no key transmission)
- 🚀 Real-time WebSocket communication
- 🔌 Claude Code extension hooks
- 📦 CLI tool for standalone use

## Installation

### Claude Code Extension

```bash
# Install as Claude Code hook
claw2claw install
```

### CLI Tool

```bash
go install github.com/epuerta9/claw2claw/cmd/claw@latest
```

## Usage

### Sending Context

```bash
# Generate a sharing code and send a file
claw send context.md

# Output: Share code: tiger-castle-neptune-7
# Share this code with the recipient
```

### Receiving Context

```bash
# Receive using the code
claw receive tiger-castle-neptune-7

# File saved to: context.md
```

### Claude Code Integration

In your Claude session, use the `/share` command:

```
/share context.md
# Generates code: tiger-castle-neptune-7

# Other user runs:
/receive tiger-castle-neptune-7
```

## Protocol

### Message Types

| Type | Description |
|------|-------------|
| `PAKE_A` | Sender's PAKE message |
| `PAKE_B` | Receiver's PAKE response |
| `ENCRYPTED` | Encrypted content payload |
| `ACK` | Acknowledgment |
| `ERROR` | Error message |

### Room Lifecycle

1. Sender creates room with code hash
2. Receiver joins with matching hash
3. PAKE exchange establishes shared key
4. Encrypted content transferred
5. Room destroyed after completion

## Configuration

```yaml
# ~/.config/claw2claw/config.yaml
relay:
  url: wss://relay.claw2claw.io
  # Or self-hosted:
  # url: wss://your-relay.example.com

encryption:
  curve: p256  # or p521 for higher security

hooks:
  claude_code: true
  auto_save: ~/.claw2claw/received/
```

## Self-Hosting Relay

See [claw2claw-app](https://github.com/epuerta9/claw2claw-app) for relay deployment.

## Development

```bash
# Build
go build -o claw ./cmd/claw

# Test
go test ./...

# Run locally with local relay
CLAW_RELAY_URL=ws://localhost:9009 ./claw send test.md
```

## License

MIT
