## 🚧 Latest Development Build

**This is a pre-release build from the latest `main` branch.**

- **Commit**: {{COMMIT_SHA}}
- **Built**: {{BUILD_DATE}}
- **Version**: {{VERSION}}

⚠️ **Warning**: This is an unstable development build. For production use, download a stable release instead.

### Installation

#### macOS

```bash
curl -L -o ship https://github.com/{{REPOSITORY}}/releases/download/latest/ship-darwin-arm64
chmod +x ship
sudo mv ship /usr/local/bin/
```

#### Linux

```bash
# AMD64
curl -L -o ship https://github.com/{{REPOSITORY}}/releases/download/latest/ship-linux-amd64
chmod +x ship
sudo mv ship /usr/local/bin/

# ARM64
curl -L -o ship https://github.com/{{REPOSITORY}}/releases/download/latest/ship-linux-arm64
chmod +x ship
sudo mv ship /usr/local/bin/
```

#### Windows

Download [`ship-windows-amd64.exe`](https://github.com/{{REPOSITORY}}/releases/download/latest/ship-windows-amd64.exe) and add it to your PATH.

### What's New?

See the [commit history](https://github.com/{{REPOSITORY}}/commits/main) for recent changes.

### Checksums

See `checksums.txt` for SHA256 checksums of all binaries.
