# go-domestia

A bridge that connects a **Domestia** lighting controller (e.g. the DMC-008) to
**Home Assistant** over MQTT. It exposes each relay on the controller as a Home
Assistant light using [MQTT discovery](https://www.home-assistant.io/integrations/mqtt/#mqtt-discovery),
so your Domestia-controlled lights appear automatically in Home Assistant with
no manual entity configuration.

## What it does

Domestia controllers speak a small binary protocol over TCP, which Home
Assistant cannot talk to directly. This bridge sits between the two:

- **Discovers lights automatically.** On startup it publishes a retained MQTT
  discovery config for each configured relay, so Home Assistant creates the
  light entities by itself.
- **Reflects controller state.** It polls the controller on a fixed interval
  and publishes on/off state and brightness to MQTT, but only when something
  actually changes.
- **Accepts commands.** It subscribes to each light's command topic and
  translates Home Assistant on/off/brightness commands into Domestia controller
  commands.
- **Supports dimmable and non-dimmable relays.** Brightness is converted
  between Home Assistant's `0–255` scale and the controller's `0–63` scale, with
  proper rounding so low brightness values dim the light instead of switching it
  off. Non-dimmable relays are forced to full brightness.
- **Supports always-on relays.** Relays marked `always_on` are kept on at full
  brightness and hidden from Home Assistant.
- **Reports availability.** Lights are marked *available* once the bridge is
  connected and registered, and *unavailable* on a clean shutdown or if the
  bridge dies unexpectedly (via the MQTT Last Will & Testament).

## How it works

```
┌──────────────┐      TCP (port 52001)      ┌──────────────┐      MQTT      ┌────────────────┐
│   Domestia   │ <────────────────────────> │  go-domestia │ <────────────> │ Home Assistant │
│  controller  │   binary relay protocol    │    bridge    │  discovery +   │   (+ broker)   │
└──────────────┘                            └──────────────┘  state/command └────────────────┘
```

- **State (controller → HA):** every `refresh_frequency` milliseconds the bridge
  sends a get-state command (`0x3c`), parses one byte per relay, and publishes
  any changes to each light's `domestia/light/<entity>/state` topic.
- **Commands (HA → controller):** the bridge subscribes to
  `domestia/light/<entity>/set`; incoming JSON commands are turned into
  turn-on (`0x0e`), turn-off (`0x0f`) or set-brightness (`0x10`) commands.
- **Availability:** published retained to `domestia/bridge/availability`
  (`online` / `offline`), also used as the MQTT Last Will.

## Tech stack

- **Go** (see `go.mod` for the exact version)
- [`eclipse/paho.mqtt.golang`](https://github.com/eclipse/paho.mqtt.golang) — MQTT client
- [`sirupsen/logrus`](https://github.com/sirupsen/logrus) — structured logging
- **Docker** (multi-arch: `armv7`, `aarch64`, `amd64`) for distribution
- Packaged as a **Home Assistant add-on** (`config.yaml`)

You will also need:

- A Domestia controller reachable over TCP on port **52001**
- An **MQTT broker** (e.g. the [Mosquitto](https://github.com/home-assistant/addons/tree/master/mosquitto)
  add-on) that Home Assistant is connected to

## Configuration

The bridge reads a single JSON file. Its path defaults to `domestia.json` and
can be overridden with the `CONFIG_PATH` environment variable (the Docker image
sets it to `/data/options.json`, which is where the Home Assistant add-on writes
its options).

Copy [`domestia.sample.json`](domestia.sample.json) to `domestia.json` and edit it:

```json
{
  "ip_address": "192.168.1.2",
  "refresh_frequency": 2000,
  "mqtt": {
    "ip_address": "192.168.1.1",
    "username": "username",
    "password": "password"
  },
  "lights": [
    { "name": "Garden", "relay": 2, "dimmable": false },
    { "name": "Living room", "relay": 13, "dimmable": true },
    { "name": "Wardrobe", "relay": 1, "always_on": true }
  ]
}
```

| Field               | Type     | Required | Description                                                            |
| ------------------- | -------- | -------- | ---------------------------------------------------------------------- |
| `ip_address`        | string   | yes      | IP address of the Domestia controller.                                 |
| `refresh_frequency` | int (ms) | no       | Controller poll interval. Defaults to `2000`. Must be greater than 0.  |
| `mqtt.ip_address`   | string   | yes      | Hostname/IP of the MQTT broker (port `1883`).                          |
| `mqtt.username`     | string   | no       | MQTT username.                                                         |
| `mqtt.password`     | string   | no       | MQTT password.                                                         |
| `lights[].name`     | string   | yes      | Display name in Home Assistant.                                        |
| `lights[].relay`    | int      | yes      | Relay number on the controller.                                        |
| `lights[].dimmable` | bool     | no       | `true` if the relay supports brightness. Defaults to `false`.          |
| `lights[].always_on`| bool     | no       | Keep the relay forced on at full brightness and hide it from HA.       |

> **Note:** `domestia.json` contains real credentials and is git-ignored. Do not
> commit it.

## Building the Docker image

The included [`Dockerfile`](Dockerfile) is a multi-stage, multi-arch build that
produces a tiny `scratch`-based image containing only the static binary.

### Single architecture (your current platform)

```sh
docker build -t go-domestia .
```

### Multi-architecture (using Buildx)

The build honours the `TARGETARCH` / `TARGETVARIANT` arguments that Buildx sets,
so you can build for the controller's target platform:

```sh
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t ghcr.io/<your-user>/go-domestia \
  --push .
```

## Running

### As a standalone Docker container

The image sets `CONFIG_PATH=/data/options.json`, so mount your configuration
there:

```sh
docker run --rm \
  -v "$(pwd)/domestia.json:/data/options.json:ro" \
  go-domestia
```

Or override `CONFIG_PATH` to mount it elsewhere:

```sh
docker run --rm \
  -e CONFIG_PATH=/config/domestia.json \
  -v "$(pwd)/domestia.json:/config/domestia.json:ro" \
  go-domestia
```

### Running locally without Docker

```sh
go build -o go-domestia .
./go-domestia                 # reads ./domestia.json
# or
CONFIG_PATH=/path/to/config.json ./go-domestia
```

The bridge logs each registered light, every state change, and any controller
or MQTT errors. It shuts down cleanly on `SIGINT`/`SIGTERM`, marking all lights
unavailable in Home Assistant before exiting.

### As a Home Assistant add-on

This repository is also a Home Assistant add-on. [`config.yaml`](config.yaml) is
the add-on manifest; Home Assistant renders its options in the UI, validates
them against the `schema:` block, and writes the result to `/data/options.json`
(which the image reads via `CONFIG_PATH`).

1. Add this repository to **Settings → Add-ons → Add-on Store → ⋮ →
   Repositories**.
2. Install the **Domestia Bridge** add-on.
3. Configure it in the add-on **Configuration** tab — see
   [`config.sample.yaml`](config.sample.yaml) for a fully worked-out example of
   the available options.
4. Start the add-on. Your lights appear automatically in Home Assistant.

## Project layout

| Path             | Responsibility                                                        |
| ---------------- | --------------------------------------------------------------------- |
| `main.go`        | Entry point: loads config, runs the bridge, handles signals/restarts. |
| `bridge/`        | Core bridge loop: MQTT wiring, polling, command handling.             |
| `domestia/`      | Domestia controller TCP client and protocol encoding.                 |
| `homeassistant/` | Home Assistant MQTT discovery and state/command payload types.        |
| `config/`        | Configuration loading and validation.                                 |
