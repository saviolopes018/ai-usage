# AI Usage Monitor

<p align="center">
  <a href="README.md">Português</a> · <strong>English</strong>
</p>

A local monitor for tracking **Codex** and **Claude Code** usage from your phone without sending provider tokens or credentials to external services.

The project combines a Go agent running on macOS with an Expo/React Native app. The agent queries locally authenticated tools, publishes authenticated snapshots over the network, and lets the app present usage, reset times, provider availability, and connection health at a glance.

> This project is under active development. It is designed for personal use on a trusted network; read [Security](#security) before exposing the agent outside your local network.

## Highlights

- Codex and Claude Code usage-window monitoring.
- Real-time updates over WebSocket.
- Automatic Mac discovery using Bonjour/mDNS.
- QR code pairing with a single-use ticket.
- Individual, revocable credentials for every device.
- Manual configuration as a fallback for local discovery.
- Separate diagnostics for network, agent, WebSocket, and providers.
- Local alerts for usage, stale data, unavailable providers, and window resets.
- Last-known-good persistence that never turns missing data into `0%`.
- Automatic light and dark themes.
- iOS Home Screen and Lock Screen widgets showing the percentage used.
- Automatic macOS service managed by `launchd`.

## How it works

```text
┌──────────────────────────── Mac ────────────────────────────┐
│                                                            │
│  Codex app-server ─┐                                       │
│                    ├─> usage-agent ─> HTTP + WebSocket      │
│  Claude statusLine ┘        │              │                │
│                             ├─ local cache  └─ Bonjour/mDNS │
│                             └─ per-device credentials       │
└─────────────────────────────────────────────┬──────────────┘
                                              │ local network
                                    ┌─────────▼─────────┐
                                    │ iOS / Android app │
                                    │ + iOS widgets     │
                                    └───────────────────┘
```

The agent keeps a single concurrent snapshot in memory and only publishes actual changes. A client immediately receives the current state when it connects; subsequent readings are distributed to WebSocket clients. Ping/pong removes inactive connections, while `SIGINT` and `SIGTERM` trigger graceful shutdown.

The app never reads Codex or Claude credentials. The agent communicates with tools already authenticated on the Mac and only exposes percentages, reset times, and operational state.

## Components

| Component | Technology | Responsibility |
|---|---|---|
| `usage-agent` | Go | Collect, store, and publish local usage |
| Mobile app | Expo 54, React Native 0.81 | Monitor, diagnostics, alerts, and setup |
| iOS widget | SwiftUI and WidgetKit | Usage summary on the Home and Lock Screens |
| Discovery | Bonjour/mDNS | Find the agent without pinning the Mac IP |
| Transport | HTTP and WebSocket | Snapshots, refresh, diagnostics, and updates |

## Requirements

### Agent

- macOS.
- Go 1.26 or a version compatible with `go.mod`.
- Codex CLI installed and authenticated to monitor Codex.
- Claude Code installed and authenticated to monitor Claude.

### Mobile app

- Node.js 24.7.0, defined in [`mobile/.nvmrc`](mobile/.nvmrc).
- npm.
- iOS: macOS, Xcode, and CocoaPods.
- Android: Android Studio/SDK and a configured device or emulator.
- A native Release build; features such as Bonjour, notifications, and widgets are not fully available in Expo Go.

### iOS widget

- iOS 17 or later for the WidgetKit extension.
- An Apple account configured in Xcode.
- App and extension signed with the App Groups capability.

## Quick start

### 1. Clone and build the agent

```bash
git clone https://github.com/saviolopes/ai-usage-monitor.git
cd ai-usage-monitor
go build -o usage-agent ./agent/cmd/usage-agent
```

Start the agent:

```bash
./usage-agent serve
```

On first run it creates:

- `~/.ai-usage/config.json`, with a random 256-bit token and `0600` permissions;
- `~/.ai-usage/`, with `0700` permissions;
- default port `9876`.

Check its status from another terminal:

```bash
./usage-agent status
```

### 2. Configure Claude Code

After moving the binary to its permanent location:

```bash
./usage-agent configure-claude
```

This preserves all existing keys in `~/.claude/settings.json`, creates `~/.claude/settings.json.ai-usage-backup`, and configures `statusLine` with the agent's absolute path.

The Codex collector needs no additional configuration. It launches `codex app-server --stdio`, negotiates the installed version, and follows rate-limit updates.

### 3. Install app dependencies

```bash
cd mobile
nvm use
npm install
```

### 4. Pair your phone

Keep the agent running and execute from the project root:

```bash
./usage-agent pair
```

Choose QR pairing in the app and scan the code shown in the terminal. The QR contains a single-use ticket, expires after five minutes, and never carries the master token.

Manual setup is also available. Enter:

- the Mac address, for example `192.168.1.20:9876`;
- the token stored in `~/.ai-usage/config.json`.

The Mac and phone must be on the same network, and Local Network access must be allowed by the operating system.

## Running the app

### iOS Simulator

```bash
cd mobile
npx expo run:ios --configuration Release
```

### Physical iPhone

Connect the iPhone to the Mac, unlock it, confirm **Trust This Computer**, and enable Developer Mode when prompted.

Find the device identifier:

```bash
xcrun xctrace list devices
```

Build and install a Release build with JavaScript embedded in the application:

```bash
cd mobile
npx expo run:ios --configuration Release --device YOUR_UDID --no-bundler
```

After installation, this build runs independently: it does not require a development server or depend on the Mac's IP address to load the interface. The first build can take a while because React Native is compiled natively. Later builds reuse the incremental cache.

### Android

With an emulator running or a USB device authorized:

```bash
cd mobile
npx expo run:android --variant release
```

## iOS signing and App Group

The project currently uses:

```text
App:       com.saviolopes.aiusagemonitor
Widget:    com.saviolopes.aiusagemonitor.widget
App Group: group.com.saviolopes.aiusagemonitor
```

To sign with another Apple account, replace these identifiers with values under your control and enable **App Groups** for both app and widget targets. Both targets must use the same group.

Only the display snapshot is shared with the widget. Endpoint, token, and device credentials are never written to the App Group.

After installing a new build:

1. Open the app at least once.
2. Wait for a valid reading.
3. Open the iOS widget gallery.
4. Search for **AI Usage**.
5. Select the compact, detailed, or Lock Screen widget.

If iOS keeps an old extension after an update, remove the widget and add it again.

## Agent commands

```text
usage-agent serve
usage-agent status
usage-agent pair
usage-agent devices
usage-agent revoke-device ID
usage-agent configure-claude
usage-agent install-service
usage-agent service-status
usage-agent uninstall-service
```

### Automatic macOS service

Install and inspect the service:

```bash
./usage-agent install-service
./usage-agent service-status
```

The installer:

- copies the binary to `~/.local/bin/usage-agent`;
- registers `~/Library/LaunchAgents/com.saviolopes.ai-usage-monitor.plist`;
- enables `RunAtLoad` and `KeepAlive`;
- connects Claude's statusLine to the installed binary;
- writes rotating logs to `~/.ai-usage/agent.log`.

Remove the service while preserving configuration and caches:

```bash
./usage-agent uninstall-service
```

### Device management

```bash
./usage-agent devices
./usage-agent revoke-device ID
```

Every pairing receives its own credential. Once an individual credential exists, the master token is accepted only from local Mac processes; LAN clients must use device tokens.

## Data states

The app keeps operational states unambiguous:

- **0% used:** a valid reading with no consumption.
- **No reading:** the usage window has not been reported.
- **Provider unavailable:** the local tool is not available or did not respond.
- **Stale data:** a valid last reading exists but is older than the freshness threshold.
- **Offline:** the app cannot reach the agent.

The agent persists the latest snapshot to `~/.ai-usage/snapshot.json`. Claude also uses `~/.ai-usage/claude-usage.json`. Both files use `0600` permissions and preserve the original observation time.

## Refresh and alerts

The app can actively request readings every 30 seconds, 1 minute, or 5 minutes. Pull-to-refresh and the header refresh action request a complete Codex and Claude update.

Local alerts are available for:

- usage thresholds;
- unavailable providers;
- stale data;
- usage-window resets;
- projected exhaustion based on recent history;
- quiet hours.

Future resets are scheduled locally by the operating system so they can fire while the app is in the background.

## Local API

The default port is `9876`.

| Method | Endpoint | Authentication | Purpose |
|---|---|---|---|
| `GET` | `/health` | Public | Process health |
| `GET` | `/state` | Bearer token | Current snapshot |
| `GET` | `/ws?protocol=1` | WebSocket subprotocol | Real-time updates |
| `POST` | `/codex/refresh` | Bearer token | Refresh Codex |
| `POST` | `/claude/refresh` | Bearer token | Refresh Claude |
| `GET` | `/auth/info` | Bearer token | Credential information |
| `GET` | `/devices` | Bearer token | Paired devices |
| `DELETE` | `/devices/{id}` | Bearer token | Revoke a device |

The client explicitly negotiates the current protocol. Incompatible versions receive HTTP `426`.

Local diagnostic request:

```bash
TOKEN=$(sed -n 's/.*"token": "\([^"]*\)".*/\1/p' ~/.ai-usage/config.json)
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:9876/state
```

Example snapshot:

```json
{
  "protocolVersion": 1,
  "agentVersion": "1.1.0",
  "capabilities": [],
  "device": "MacBook",
  "online": true,
  "updatedAt": "2026-08-08T18:00:00Z",
  "providers": [
    {
      "provider": "codex",
      "available": true,
      "observedAt": "2026-08-08T18:00:00Z",
      "weekly": {
        "usedPercentage": 31,
        "remainingPercentage": 69,
        "resetsAt": "2026-08-15T18:00:00Z"
      }
    }
  ]
}
```

## Security

- Provider tokens never leave their tools or reach the app.
- The master token contains 256 random bits.
- Sensitive local files use restrictive permissions.
- QR pairing uses a single-use ticket with a five-minute expiration.
- Every phone receives a separate, revocable credential.
- mDNS announcements contain no token.
- The internal statusLine endpoint only accepts direct localhost requests.
- The widget receives no endpoint, token, or credential.
- Authentication tokens are not written to logs.

HTTP on the local network does not provide transport encryption. Do not forward port `9876` directly to the internet. For remote access, use a trusted private network/VPN and individual device credentials.

## Development and quality

From the repository root:

```bash
go test ./...
go vet ./...
```

For the mobile app:

```bash
cd mobile
npm run lint
npm run typecheck
npm test
npx expo-doctor
```

The project includes tests for configuration, collectors, caching, mDNS, pairing, HTTP/WebSocket behavior, notifications, and visual states.

## Repository structure

```text
agent/
  cmd/usage-agent/       CLI and process lifecycle
  internal/claude/       Claude statusLine, refresh, and cache
  internal/codex/        app-server, JSON-RPC, and rate limits
  internal/config/       configuration and credentials
  internal/domain/       snapshot model
  internal/launchd/      automatic macOS service
  internal/logging/      rotating logs
  internal/mdns/         Bonjour advertising
  internal/pairing/      pairing tickets and QR codes
  internal/server/       HTTP, authentication, and WebSocket
  internal/store/        concurrent state and subscriptions
mobile/
  src/components/        reusable visual components
  src/screens/           monitor, diagnostics, alerts, and setup
  ios/AIUsageWidget/     WidgetKit extension
  App.tsx                navigation and main composition
docs/                    audits and additional documentation
```

## Troubleshooting

### `No space left on device` during an Xcode build

Check available storage:

```bash
df -h /
du -sh ~/Library/Developer/Xcode/DerivedData 2>/dev/null
```

Close Xcode and remove only the required regenerable caches, such as this project's DerivedData, ModuleCache, CocoaPods cache, and npm package cache. Do not remove DeviceSupport or simulator data without inspecting them first.

### The iPhone shows the “Development Servers” screen

- A development build is installed on the device.
- Reinstall with `npx expo run:ios --configuration Release --device YOUR_UDID --no-bundler`.
- The Release build loads its embedded bundle and does not look for a server on port `8081`.

### The app cannot find the agent

- Check `./usage-agent status`.
- Open `http://MAC_IP:9876/health` from the phone.
- Allow Local Network and Bonjour access.
- Make sure port `9876` is not blocked.
- Use QR or manual setup to rule out discovery issues.

### Claude has no reading

Run:

```bash
./usage-agent configure-claude
```

Then open Claude Code and produce a new status update, or use **Refresh** in the app.

### The widget does not update

- Open the main app and wait for a valid reading.
- Confirm that the app and widget use the same App Group.
- Remove and add the widget again after installing a new build.

### Widget signing fails

Select the same team for the app and extension targets in Xcode. Confirm that the bundle identifiers are unique and both provisioning profiles include the configured App Group.

## Privacy

Snapshots, device credentials, preferences, and history stay between the configured Mac and phone. The project includes no remote backend, analytics, or cloud synchronization.

## Roadmap

- Simpler distribution of signed builds.
- Improved agent packaging and updates.
- Additional visual QA coverage across tablets and larger text sizes.
- Optional remote transport with end-to-end encryption.

Contributions and bug reports are welcome through GitHub Issues. Remove tokens, private IP addresses, device names, and sensitive log content before submitting a report.
