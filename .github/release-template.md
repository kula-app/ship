## Ship v{{VERSION}}

### Installation

#### macOS (Homebrew)

```bash
brew install kula-app/tap/ship
```

#### macOS (Manual)

```bash
curl -L -o ship https://github.com/{{REPOSITORY}}/releases/download/v{{VERSION}}/ship-darwin-arm64
chmod +x ship
sudo mv ship /usr/local/bin/
```

#### Linux

```bash
# AMD64
curl -L -o ship https://github.com/{{REPOSITORY}}/releases/download/v{{VERSION}}/ship-linux-amd64
chmod +x ship
sudo mv ship /usr/local/bin/

# ARM64
curl -L -o ship https://github.com/{{REPOSITORY}}/releases/download/v{{VERSION}}/ship-linux-arm64
chmod +x ship
sudo mv ship /usr/local/bin/
```

#### Windows

Download [`ship-windows-amd64.exe`](https://github.com/{{REPOSITORY}}/releases/download/v{{VERSION}}/ship-windows-amd64.exe) and add it to your PATH.

### Usage

```bash
# Authenticate with Shipable
ship auth login
```

See the [README](https://github.com/{{REPOSITORY}}/blob/main/README.md) for more details.

### Checksums

See `checksums.txt` for SHA256 checksums of all binaries.
