# Release Notes

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v0.12.0] - 2026-03-24

### Added

- **Git submodule support**: comin now fetches and initializes Git submodules
  ([#141](https://github.com/nlewo/comin/pull/141)) — Lucas ([@Keyruu](https://github.com/Keyruu))

### Contributors

- lewo ([@nlewo](https://github.com/nlewo))
- Lucas ([@Keyruu](https://github.com/Keyruu))

## [v0.11.0] - 2026-02-18

### Added

- New gauge metrics `comin_is_suspended` and `comin_need_to_reboot` to follow
  Prometheus best-practices.

### Deprecated

- `comin_host_info` is deprecated and will be removed in the next release. Use
  `comin_need_to_reboot` and `comin_is_suspended` instead.
