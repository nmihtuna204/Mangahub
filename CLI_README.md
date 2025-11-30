# MangaHub CLI Tool

A powerful command-line interface for the MangaHub manga tracking system.

## Installation

### Build from Source
```bash
# Clone the repository
git clone https://github.com/yourusername/mangahub.git
cd mangahub

# Build the CLI
make build-cli

# The executable will be in bin/mangahub.exe (Windows) or bin/mangahub (Linux/Mac)
```

## Quick Start

### 1. Register a New Account
```bash
./bin/mangahub.exe auth register --username myuser --email my@email.com
# Enter password when prompted
```

### 2. Login
```bash
./bin/mangahub.exe auth login --username myuser
# Enter password when prompted
```

### 3. Search for Manga
```bash
./bin/mangahub.exe manga search "naruto" --limit 10
```

### 4. Add Manga to Your Library
```bash
./bin/mangahub.exe library add --manga-id manga-001 --status reading
```

### 5. View Your Library
```bash
./bin/mangahub.exe library list
```

### 6. Update Reading Progress (Triggers All 5 Protocols!)
```bash
./bin/mangahub.exe progress update --manga-id manga-001 --chapter 50 --rating 9
```

## Commands Reference

### Authentication
```bash
# Register new user
mangahub auth register --username <name> --email <email>

# Login
mangahub auth login --username <name>
```

### Manga Discovery
```bash
# Search manga
mangahub manga search <query> [--limit <n>] [--status <status>]

# Examples:
mangahub manga search "one piece" --limit 5
mangahub manga search "naruto" --status completed
```

### Library Management
```bash
# Add manga to library
mangahub library add --manga-id <id> [--status <status>] [--chapter <n>]

# List your library
mangahub library list

# Examples:
mangahub library add --manga-id manga-001 --status reading --chapter 10
mangahub library list
```

### Reading Progress
```bash
# Update progress (syncs across all 5 protocols!)
mangahub progress update --manga-id <id> --chapter <n> [--rating <r>] [--status <s>]

# Examples:
mangahub progress update --manga-id manga-001 --chapter 75 --rating 9
mangahub progress update --manga-id manga-002 --chapter 100 --status completed
```

### Configuration
```bash
# Show current configuration
mangahub config show

# Show version
mangahub version
```

## Global Flags

```bash
--config <path>    # Specify config file (default: ~/.mangahub/config.yaml)
--verbose          # Enable verbose output
-h, --help         # Show help
```

## Configuration File

The CLI stores configuration in `~/.mangahub/config.yaml`:

```yaml
server:
  host: localhost
  http_port: 8080
  tcp_port: 9090
  udp_port: 9091
  grpc_port: 9092

user:
  id: 12345
  username: alice
  token: eyJhbGci...  # JWT token
```

## Multi-Protocol Synchronization

When you update reading progress, the CLI triggers **all 5 network protocols** simultaneously:

```bash
$ mangahub progress update --manga-id manga-001 --chapter 75 --rating 9

✓ Progress updated successfully!
  Manga ID: manga-001
  Chapter: 75
  Rating: 9/10

🔄 Synced across all protocols:
  ✓ HTTP: API updated
  ✓ TCP: Broadcasted to sync clients
  ✓ UDP: Notification sent
  ✓ WebSocket: Room members notified
  ✓ gRPC: Audit logged
```

This demonstrates the power of MangaHub's Protocol Bridge - a single CLI command triggers synchronized updates across HTTP, TCP, UDP, WebSocket, and gRPC!

## Prerequisites

Before using the CLI, ensure:
1. MangaHub server is running
2. PostgreSQL database is accessible
3. All protocol servers are active (HTTP, TCP, UDP, WebSocket, gRPC)

## Development

### Build
```bash
make build-cli
```

### Run without Building
```bash
make run-cli
```

### Clean Build Artifacts
```bash
make clean
```

### Run Tests
```bash
make test
```

## Examples

### Complete Workflow
```bash
# 1. Register
./bin/mangahub.exe auth register --username alice --email alice@example.com

# 2. Login
./bin/mangahub.exe auth login --username alice

# 3. Search for manga
./bin/mangahub.exe manga search "naruto"

# 4. Add to library
./bin/mangahub.exe library add --manga-id manga-001 --status reading

# 5. Update progress
./bin/mangahub.exe progress update --manga-id manga-001 --chapter 50 --rating 9

# 6. View library
./bin/mangahub.exe library list

# 7. Check config
./bin/mangahub.exe config show
```

## Troubleshooting

### "not logged in" Error
```bash
# Run login command
mangahub auth login --username youruser
```

### "connection refused" Error
```bash
# Ensure the MangaHub server is running
# Check server status and ports in config
mangahub config show
```

### "Invalid credentials" Error
```bash
# Verify username and password
# Or register a new account
mangahub auth register --username newuser --email new@example.com
```

## Features

- ✅ **User Authentication**: Secure registration and login with JWT tokens
- ✅ **Manga Search**: Find manga by title, author, or status
- ✅ **Library Management**: Track your reading list
- ✅ **Progress Tracking**: Update chapters and ratings
- ✅ **Multi-Protocol Sync**: Updates propagate across all 5 protocols
- ✅ **Persistent Config**: Settings saved between sessions
- ✅ **Secure Passwords**: Hidden input for passwords
- ✅ **User-Friendly Output**: Clear formatting with ✓ symbols

## Documentation

- Full documentation: `docs/PHASE8_SUMMARY.md`
- API documentation: `docs/API_DOCUMENTATION.md`
- Protocol details: `docs/PHASE7_SUMMARY.md`

## License

MIT License

## Support

For issues or questions, please open an issue on GitHub.
