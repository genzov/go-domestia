# Changelog

## 1.2.2

- Retained discovery configs left behind by renaming a light are removed on
  startup. They shared the light's unique ID, which could keep a stale entity
  around and stop the current one from being updated.
- The internal config topic is no longer included in the discovery payload.

After updating, restart Home Assistant once so existing light entities are
attached to their devices.

## 1.2.1

- A slow or unreachable controller no longer restarts the bridge (which
  reconnected MQTT and re-registered every light). Failed polls are retried,
  and lights are marked unavailable after 3 consecutive failures until the
  controller responds again.
- Controller requests now time out after 3 seconds (was 1 second, with no
  connect timeout).

## 1.2.0

- Each light is registered as its own Home Assistant device, named after the
  light and reporting the add-on version. Existing entities keep their IDs and
  are attached to their device automatically.

## 1.1.0

- Correct brightness scaling: Home Assistant's 0–255 brightness is now rounded
  onto the controller's 0–63 scale, and low values dim the light instead of
  switching it off.
- Graceful shutdown with availability reporting (online/offline + MQTT Last
  Will) so lights are marked unavailable when the bridge stops or dies.
- Controller and MQTT errors are logged instead of silently ignored; per-light
  state logging now reports on/off and dim level.
- Configuration is validated at startup.
- Published aarch64 image for Raspberry Pi 3 (64-bit), plus docs, sample
  configuration and store metadata.

## 1.0.0

- Initial release.
