# Changelog

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
