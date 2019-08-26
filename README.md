# Nacelle Base Worker Process [![GoDoc](https://godoc.org/github.com/go-nacelle/workerbase?status.svg)](https://godoc.org/github.com/go-nacelle/workerbase) [![CircleCI](https://circleci.com/gh/go-nacelle/workerbase.svg?style=svg)](https://circleci.com/gh/go-nacelle/workerbase) [![Coverage Status](https://coveralls.io/repos/github/go-nacelle/workerbase/badge.svg?branch=master)](https://coveralls.io/github/go-nacelle/workerbase?branch=master)

Abstract worker process for nacelle.

---

A **worker** is a process that periodically polls an external resource in order to discover or perform its main unit of work. A **base worker** is an abstract [process](https://nacelle.dev/docs/core/process) whose behavior can be be configured by implementing the `WorkerSpec` interface.

This library comes with an [example](https://github.com/go-nacelle/workerbase/tree/master/example) project. You can see an additional example of a worker process in the [example repository](https://github.com/go-nacelle/example), specifically the [worker spec](https://github.com/go-nacelle/example/blob/843979aaa86786784a1ca3646e8d0d1f69e29c65/cmd/worker/worker_spec.go#L15) definition.

### Process

A worker process is created by supplying a specification, described [below](https://nacelle.dev/docs/base-processes/workerbase#worker-specification), that controls its behavior.

```go
worker := workerbase.NewWorker(NewWorkerSpec(), options...)
```

### Worker Specification

A worker specification is a struct with an `Init` and a `Tick` method. The initialization method, like the process that runs it, that takes a config object as a parameter. The tick method takes a context object as a parameter. On process shutdown, this context object is cancelled so that any long-running work in the tick method can be cleanly abandoned. Each method may return an error value, which signals a fatal error to the process that runs it.

The following example uses a database connection injected by the service container, and pings it to logs its latency. The worker process will call the tick method in a loop based on its interval configuration while the process remains active.

```go
type Spec struct {
    DB     *sqlx.DB       `service:"db"`
    Logger nacelle.Logger `service:"logger"`
}

func (s *Spec) Init(config nacelle.Config) error {
    return nil
}

func (s *Spec) Tick(ctx context.Context) error {
    start := time.Now()
    err := s.DB.Ping()
    duration := time.Now().Sub(start)
    durationMs := float64(duration) / float64(time.Milliseconds)

    if err != nil {
        return err
    }

    s.Logger.Debug("Ping took %.2fms", durationMs)
}
```

#### Finalization

If the worker specification also implements the `Finalize` method, it will be called after the last invocation of the tick method (regardless of its return value).

```go
func (s *Spec) Finalize() error {
    s.Logger.Debug("Shutting down")
    return nil
}
```

### Worker Process Options

The following options can be supplied to the worker process instance on construction.

<dl>
  <dt>WithTagModifiers</dt>
  <dd><a href="https://godoc.org/github.com/go-nacelle/workerbase#WithTagModifiers">WithTagModifiers</a> registers the tag modifiers to be used when loading process configuration (see <a href="https://godoc.org/github.com/go-nacelle/workerbase#Configuration">below</a>). This can be used to change the default tick interval, or prefix all target environment variables in the case where more than one worker process is registered per application.</dd>
</dl>

### Configuration

The default process behavior can be configured by the following environment variables.

| Environment Variable | Default | Description |
| -------------------- | ------- | ----------- |
| WORKER_STRICT_CLOCK  | false   | Subtract the duration of the previous tick from the time between calls to the spec's tick function. |
| WORKER_TICK_INTERVAL | 0       | The time (in seconds) between calls to the spec's tick function. |
