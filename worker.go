package workerbase

import (
	"context"
	"sync"
	"time"

	"github.com/efritz/glock"
	"github.com/go-nacelle/nacelle"
	"github.com/google/uuid"
)

type (
	Worker struct {
		Services     nacelle.ServiceContainer `service:"services"`
		Health       nacelle.Health           `service:"health"`
		tagModifiers []nacelle.TagModifier
		spec         WorkerSpec
		clock        glock.Clock
		halt         chan struct{}
		once         *sync.Once
		tickInterval time.Duration
		healthToken  healthToken
	}

	WorkerSpec interface {
		Init(nacelle.Config, *Worker) error
		Tick(ctx context.Context) error
	}
)

func NewWorker(spec WorkerSpec, configs ...ConfigFunc) *Worker {
	return newWorker(spec, glock.NewRealClock())
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

func (w *Worker) IsDone() bool {
	select {
	case <-w.HaltChan():
		return true
	default:
		return false
	}
}

func (w *Worker) HaltChan() <-chan struct{} {
	return w.halt
}

func (w *Worker) Init(config nacelle.Config) error {
	if err := w.Health.AddReason(w.healthToken); err != nil {
		return err
	}

	workerConfig := &Config{}
	if err := config.Load(workerConfig, w.tagModifiers...); err != nil {
		return err
	}

	w.tickInterval = workerConfig.WorkerTickInterval

	if err := w.Services.Inject(w.spec); err != nil {
		return err
	}

	return w.spec.Init(config, w)
}

func (w *Worker) Start() error {
	defer w.Stop()

	if err := w.Health.RemoveReason(w.healthToken); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-w.halt
		cancel()
	}()

	for {
		if err := w.spec.Tick(ctx); err != nil {
			return err
		}

		select {
		case <-w.halt:
			return nil
		case <-w.clock.After(w.tickInterval):
		}
	}
}

func (w *Worker) Stop() (err error) {
	w.once.Do(func() { close(w.halt) })
	return
}
