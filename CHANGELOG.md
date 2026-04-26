# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `flagtype.EnumDefault` constructor for enums with an initial default value
- New `usage` package with composable help document blocks: `Text`, `Lines`, `List`, `Flags`, and
  `Commands`
- `Help` function for rendering the resolved command help document
- `Cmd` field on `State` for accessing the terminal command selected by parsing
- `UsageErrorf` for opt-in usage errors; `Run` prints command help to stderr before returning the
  underlying error

### Changed

- **BREAKING**: Replace `Command.UsageFunc` with `Command.Help`, which receives the built-in
  `usage.Help` document and returns the customized document
- **BREAKING**: Custom help now composes `usage.Help` documents instead of string-concatenating
  default usage text
- **BREAKING**: Rename `FlagOption` to `FlagConfig` and `Command.FlagOptions` to
  `Command.FlagConfigs`
- Help output is now built through the `usage` package while preserving the default automatic
  `--help` behavior

### Removed

- **BREAKING**: Remove `DefaultUsage` and `Usage`; use `Help(cmd).String()` for direct rendering
- **BREAKING**: Remove usage-related `State` helpers: `Command`, `CommandPath`, `Usage`, and
  `UsageErrorf`; use `State.Cmd`, `Command.Path`, `Help`, and top-level `UsageErrorf` instead

## [v0.6.0] - 2026-02-18

### Added

- New `flagtype` package with common `flag.Value` implementations: `StringSlice`, `Enum`,
  `StringMap`, `URL`, and `Regexp`

## [v0.5.0] - 2026-02-17

### Changed

- **BREAKING**: Rename `FlagMetadata` to `FlagOption` and `FlagsMetadata` field to `FlagOptions`

## [v0.4.0] - 2026-02-16

### Added

- `Local` field on `FlagOption` for granular control over flag inheritance -- flags marked local are
  not inherited by subcommands
- Short flag aliases via `FlagOption.Short`
- `ParseAndRun` convenience function for parsing and running a command in one call

### Changed

- Enhanced usage output with type hints, required markers, and zero-default suppression
- Improved `ParseToEnd` and internal flag parsing with edge case fixes

### Fixed

- Default nil context to `context.Background` in `Run`

## [v0.3.0] - 2025-11-22

### Added

- Top-level `graceful` package for signal handling
- Comprehensive edge case tests for CLI library

### Changed

- Use sync.Once to get module name from runtime
- Improve panic location reporting

## [v0.2.1] - 2025-02-01

### Changed

- Update name regex to allow underscore and dash in command names

## [v0.2.0] - 2025-01-07

### Removed

- Remove `ParseAndRun` in favor of separate parse and run steps

## [v0.1.0] - 2025-01-06

### Added

- Initial release of the CLI library
- Command tree with subcommands and flag parsing
- `Path` method on `*Command` for full command path
- Flag metadata with required flag support
- Command name typo suggestions (Levenshtein distance)
- Boolean flag handling
- `textutil` and `suggest` helper packages
- GitHub Actions CI

[Unreleased]: https://github.com/pressly/cli/compare/v0.6.0...HEAD
[v0.6.0]: https://github.com/pressly/cli/compare/v0.5.0...v0.6.0
[v0.5.0]: https://github.com/pressly/cli/compare/v0.4.0...v0.5.0
[v0.4.0]: https://github.com/pressly/cli/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/pressly/cli/compare/v0.2.1...v0.3.0
[v0.2.1]: https://github.com/pressly/cli/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/pressly/cli/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/pressly/cli/releases/tag/v0.1.0
