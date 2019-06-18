# Nacelle Base Worker Process [![GoDoc](https://godoc.org/github.com/go-nacelle/workerbase?status.svg)] [![CircleCI](https://circleci.com/gh/go-nacelle/workerbase.svg?style=svg)](https://circleci.com/gh/go-nacelle/workerbase) [![Coverage Status](https://coveralls.io/repos/github/go-nacelle/workerbase/badge.svg?branch=master)](https://coveralls.io/github/go-nacelle/workerbase?branch=master)

Abstract worker process for nacelle.

---

### Usage

The supplied process is an abstract busy-loop whose behavior is determined by a supplied `WorkerSpec` interface. This interface has an `Init` method that receives application config as well as the worker process instance and a `Tick` method where each phase of work should be done. The worker process has methods and `IsDone` and `HaltChan` which can be used within the tick method to determine if long-running work should be abandoned on application shutdown. There is an [example](./example) included in this repository.

- **WithTagModifiers** registers the tag modifiers to be used when loading process configuration (see [below](#Configuration)). This can be used to change the default tick interval, or prefix all target environment variables in the case where more than one worker process is registered per application.

### Configuration

The default process behavior can be configured by the following environment variables.

| Environment Variable | Default | Description |
| -------------------- | ------- | ----------- |
| WORKER_TICK_INTERVAL | 0       | The time (in seconds) between calls to teh spec's tick function. |
