package workerbase

import (
	"context"
	"sync"
	"time"

	"github.com/derision-test/glock"
	"github.com/go-nacelle/nacelle"
	"github.com/google/uuid"
)

type Worker struct {
	Services     nacelle.ServiceContainer `service:"services"`
	Health       nacelle.Health           `service:"health"`
	tagModifiers []nacelle.TagModifier
	spec         WorkerSpec
	clock        glock.Clock
	halt         chan struct{}
	once         *sync.Once
	tickInterval time.Duration
	strictClock  bool
	healthToken  healthToken
}

type WorkerSpec interface {
	Init(nacelle.Config) error
	Tick(ctx context.Context) error
}

type workerSpecFinalizer interface {
	nacelle.Finalizer
	WorkerSpec
}

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
		once:         &sync.Once{},
		healthToken:  healthToken(uuid.New().String()),
	}
}

func (w *Worker) Init(config nacelle.Config) error {
	if err := w.Health.AddReason(w.healthToken); err != nil {
		return err
	}

	workerConfig := &Config{}
	if err := config.Load(workerConfig, w.tagModifiers...); err != nil {
		return err
	}

	w.strictClock = workerConfig.StrictClock
	w.tickInterval = workerConfig.WorkerTickInterval

	if err := w.Services.Inject(w.spec); err != nil {
		return err
	}

	return w.spec.Init(config)
}

func (w *Worker) Start() (err error) {
	if finalizer, ok := w.spec.(nacelle.Finalizer); ok {
		defer func() {
			finalizeErr := finalizer.Finalize()
			if err == nil {
				err = finalizeErr
			}
		}()
	}

	defer w.Stop()

	if err = w.Health.RemoveReason(w.healthToken); err != nil {
		return
	}

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

func (w *Worker) Stop() error {
	w.once.Do(func() { close(w.halt) })
	return nil
}
