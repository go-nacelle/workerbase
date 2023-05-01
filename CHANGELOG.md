# Changelog

## [Unreleased]

### Removed

- `Worker.Config` field removed in favor of [`config.LoadFromContext`](https://pkg.go.dev/github.com/go-nacelle/config/v3#LoadFromContext).
- `Worker.Services` field removed in favor of [`service.FromContext`](https://pkg.go.dev/github.com/go-nacelle/service/v2#FromContext).
- `Worker.Health` field removed in favor of [`process.HealthFromContext`](https://pkg.go.dev/github.com/go-nacelle/process/v2#HealthFromContext).

### Changed

- `WorkerSpec.Init` parameter changed to `context.Context` to match Nacelle. [#3](https://github.com/go-nacelle/workerbase/pull/3)
- Update dependency [go-nacelle/nacelle@v1.0.2] -> [go-nacelle/nacelle@v2.1.0]

[unreleased]: https://github.com/go-nacelle/workerbase/compare/v1.2.0...HEAD
[go-nacelle/nacelle@v1.0.2]: https://github.com/go-nacelle/nacelle/releases/tag/v1.0.2
[go-nacelle/nacelle@v2.1.0]: https://github.com/go-nacelle/nacelle/releases/tag/v2.1.0

