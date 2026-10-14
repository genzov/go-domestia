# AGENTS.md

go-domestia bridges a Domestia lighting controller (binary protocol over TCP port 52001) to Home Assistant over MQTT discovery. It ships as a static Go binary in a `scratch` Docker image and as a Home Assistant add-on (`go_domestia/`).

## Commands

```sh
go build -o go-domestia .                 # build
go test ./...                             # all tests
go test ./bridge -run TestOrphanedTopics  # single test
go vet ./...
gofmt -l .                                # CI fails if this prints anything
CONFIG_PATH=domestia.json ./go-domestia   # run locally (CONFIG_PATH defaults to ./domestia.json)
docker build --build-arg VERSION=dev -t go-domestia .
```

CI (`.github/workflows/ci.yml`) runs gofmt, `go vet`, `go test`, and a multi-arch Docker build (arm64, arm/v7, amd64). The binary's version is injected with `-ldflags "-X main.version=<version>"` and reported to Home Assistant as the device `sw_version`.

`domestia.json` is git-ignored and holds real controller IPs and MQTT credentials. Never commit it. Use `domestia.sample.json` as the template.

## Architecture

Package dependencies: `main` → `bridge` → `domestia`, `config`, `homeassistant`; `domestia` → `config`; `config` → `homeassistant`.

- **`main.go`**: loads config, then runs `bridge.Run` in a restart loop with a 1s backoff. `Run` returns nil only when the context is cancelled (SIGINT/SIGTERM); any error restarts it.
- **`bridge/`**: the run loop polls the controller on a ticker, or immediately when a command arrives through `updateChannel`. The channel is buffered to 1 and written non-blocking so refreshes coalesce. State is published only when a relay's brightness changes (`relayToBrightness`).
  - Controller errors never return from `Run`. They are retried on the next tick, and after 3 consecutive failures the lights are marked unavailable. MQTT publish errors do return, which triggers a full restart.
  - Discovery registration and command subscriptions happen in the MQTT `OnConnectHandler` so they are redone after auto-reconnect. That handler runs on paho's goroutine: log errors there, never exit.
  - Each poll also enforces policy: `always_on` relays are forced back to full brightness (they are never registered with HA), and a non-dimmable relay found dimmed is reset to max.
  - On startup, `removeOrphanedConfigs` collects retained `homeassistant/light/+/config` messages for 2s and clears any that carry one of our `unique_id`s under a stale topic. This happens after a light is renamed.
- **`domestia/`**: every request opens a fresh TCP connection (3s timeout), serialized by a mutex. Frames are `0xff 0x00 0x00 <len> <cmd...> <checksum>`, where the checksum is the byte sum of cmd. Commands: `0x3c` get state, `0x0e` on, `0x0f` off, `0x10` brightness. Commands reply `"OK"`. Get-state replies `0xff`, then two status bytes, then one byte per relay (relay number = index + 1).
- **`homeassistant/`**: MQTT payload types and topic naming. The entity id is the light name lowercased with spaces replaced by `_`, and it forms the config, command, and state topics. One bridge-wide availability topic, `domestia/bridge/availability`, doubles as the MQTT Last Will. A graceful disconnect suppresses the will, so the bridge publishes `offline` itself on shutdown.
- **`config/`**: JSON loading and validation. `config.Light` builds the discovery payload: `unique_id` is `d_<relay>` and the device identifier is `domestia_relay_<relay>`. Changing either breaks existing entities in users' Home Assistant installs.

### Brightness

The controller scale is 0–`domestia.MaxBrightness` (64; a fully-on relay reads back 64). HA uses 0–255. Conversions in `bridge/light_state.go` round rather than truncate. A non-zero HA brightness is floored to 1 so it dims instead of switching off. A non-dimmable relay reports `1` when on, and `NewLight` (`domestia/light.go`) normalizes that to max. For dimmable relays, `1` is a real dim level.
### Tests

There are no MQTT or controller fakes. Testable logic is extracted into pure functions (`orphanedTopics`, the brightness conversions, `NewLight`, `packCommand`, config loading) and tested table-style. Follow that pattern rather than mocking clients.

## Changing configuration options

A new or changed option has to be updated in all of these places:

- `config/configuration.go` (struct and validation)
- `go_domestia/config.yaml` (`options` and `schema`; HA validates against `schema` and writes `/data/options.json`)
- `go_domestia/translations/en.yaml`
- `go_domestia/DOCS.md`
- the README configuration table
- `domestia.sample.json`

## Releasing

To release, bump `version` in `go_domestia/config.yaml` and add a matching `## <version>` section to `go_domestia/CHANGELOG.md`. The release workflow refuses to run without that section. Merging to `main` with a changed `config.yaml` triggers `release.yml`, which tests, pushes `ghcr.io/genzov/go-domestia-<arch>:<version>` and `:latest`, and creates a GitHub release from the changelog section. Already-tagged versions are skipped. Changelog entries are written for add-on users (behavior they will notice), not as commit summaries.

## Conventions

Logging uses `log/slog` with a capitalized message and key=value attributes, e.g. `slog.Info("Light state changed", "light", name, "state", s)`.
