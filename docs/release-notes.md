# Release Notes

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- New gauge metrics `comin_is_suspended` and `comin_need_to_reboot` to follow
  Prometheus best-practices.

### Deprecated

- `comin_host_info` is deprecated and will be removed in the next release. Use
  `comin_need_to_reboot` and `comin_is_suspended` instead.
