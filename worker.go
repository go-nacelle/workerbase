package workerbase

import (
	"context"
	"sync"
	"time"

	"github.com/derision-test/glock"
	"github.com/go-nacelle/config/v3"
	"github.com/go-nacelle/nacelle/v2"
	"github.com/go-nacelle/process/v2"
	"github.com/go-nacelle/service/v2"
	"github.com/google/uuid"
)

type (
	Worker struct {
		tagModifiers []nacelle.TagModifier
		spec         WorkerSpec
		clock        glock.Clock
		halt         chan struct{}
		done         chan struct{}
		once         *sync.Once
		tickInterval time.Duration
		strictClock  bool
		healthToken  healthToken
		healthStatus *process.HealthComponentStatus
	}

	WorkerSpec interface {
		Init(ctx context.Context) error
		Tick(ctx context.Context) error
	}

	workerSpecFinalizer interface {
		process.Finalizer
		WorkerSpec
	}
)

func NewWorker(spec WorkerSpec, configs ...ConfigFunc) *Worker {
	return newWorker(spec, glock.NewRealClock(), configs...)
}

func newWorker(spec WorkerSpec, clock glock.Clock, configs ...ConfigFunc) *Worker {
	options := getOptions(configs)

	return &Worker{
		tagModifiers: options.tagModifiers,
		spec:         spec,
		clock:        clock,
		halt:         make(chan struct{}),
		done:         make(chan struct{}),
		once:         &sync.Once{},
		healthToken:  healthToken(uuid.New().String()),
	}
}

func (w *Worker) Init(ctx context.Context) error {
	health := process.HealthFromContext(ctx)
	healthStatus, err := health.Register(w.healthToken)
	if err != nil {
		return err
	}
	w.healthStatus = healthStatus

	workerConfig := &Config{}
	if err := config.LoadFromContext(ctx, workerConfig, w.tagModifiers...); err != nil {
		return err
	}

	w.strictClock = workerConfig.StrictClock
	w.tickInterval = workerConfig.WorkerTickInterval

	svc := service.FromContext(ctx)
	if err := service.Inject(ctx, svc, w.spec); err != nil {
		return err
	}

	return w.spec.Init(ctx)
}

func (w *Worker) Run(ctx context.Context) (err error) {
	if finalizer, ok := w.spec.(nacelle.Finalizer); ok {
		defer func() {
			finalizeErr := finalizer.Finalize(ctx)
			if err == nil {
				err = finalizeErr
			}
		}()
	}

	defer w.Stop(ctx)

	w.healthStatus.Update(true)
	defer close(w.done)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-w.halt
		cancel()
	}()

	for {
		started := w.clock.Now()
		if err = w.spec.Tick(ctx); err != nil {
			return
		}

		interval := w.tickInterval
		if w.strictClock {
			interval -= w.clock.Now().Sub(started)
		}

		select {
		case <-w.halt:
			return
		case <-w.clock.After(interval):
		}
	}
}

func (w *Worker) Stop(ctx context.Context) error {
	w.once.Do(func() { close(w.halt) })
	<-w.done
	return nil
}
