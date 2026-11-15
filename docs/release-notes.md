# Release Notes

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v0.14.0] - 2025-07-19

### Added

- **SSH commit signature verification**: Support for verifying Git commits signed with SSH keys ([#171](https://github.com/nlewo/comin/pull/171)
- **SSH key authentication**: Support for SSH key authentication for Git operations ([#168](https://github.com/nlewo/comin/pull/168))
- Customizable build and evaluation timeouts ([#174](https://github.com/nlewo/comin/pull/174))
- Support for switch inhibitors ([#156](https://github.com/nlewo/comin/pull/156))

### Changed

- `deployent submit-latest` command replacing `switch-latest` ([#152](https://github.com/nlewo/comin/pull/152))
- Use bare local repository for git operations ([#154](https://github.com/nlewo/comin/pull/154))

### Contributors

- Antoine Eiche ([@nlewo](https://github.com/nlewo))
- Arunesh Dwivedi ([@AruneshDwivedi](https://github.com/AruneshDwivedi))
- Bas Nijholt ([@basnijholt](https://github.com/basnijholt))
- Cameron Ackerman ([@camja014](https://github.com/camja014))
- Eli Saado ([@elisaado](https://github.com/elisaado))
- Krzysztof Kuśmierczyk ([@krzysztofkusmierczyk](https://github.com/krzysztofkusmierczyk))
- Lucas ([@Keyruu](https://github.com/Keyruu))

## [v0.13.0] - 2026-05-07

### Added

- `comin watch` command for monitoring repositories
- Fetcher support to watch functionality
- Support for proxies ([#150](https://github.com/nlewo/comin/pull/150))

### Changed

- Use a bare local repository for git operations ([#154](https://github.com/nlewo/comin/pull/154))
- CLI: improved messages ([#145](https://github.com/nlewo/comin/pull/145))

### Fixed

- Desktop: fix typo ([#145](https://github.com/nlewo/comin/pull/145))

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
