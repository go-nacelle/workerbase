package workerbase

import (
	"context"
	"fmt"
	"time"

	"github.com/aphistic/sweet"
	"github.com/efritz/glock"
	"github.com/go-nacelle/nacelle"
	. "github.com/onsi/gomega"
)

type WorkerSuite struct{}

var testConfig = nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
	"worker_tick_interval": "5",
}))

func (s *WorkerSuite) TestRunAndStop(t sweet.T) {
	var (
		spec     = NewMockWorkerSpec()
		clock    = glock.NewMockClock()
		worker   = makeWorker(spec, clock)
		tickChan = make(chan struct{})
		errChan  = make(chan error)
	)

	defer close(tickChan)

	spec.TickFunc.SetDefaultHook(func(ctx context.Context) error {
		tickChan <- struct{}{}
		return nil
	})

	err := worker.Init(testConfig)
	Expect(err).To(BeNil())

	go func() {
		errChan <- worker.Start()
	}()

	Eventually(tickChan).Should(Receive())
	Consistently(tickChan).ShouldNot(Receive())
	clock.BlockingAdvance(time.Second * 5)
	Eventually(tickChan).Should(Receive())
	Consistently(tickChan).ShouldNot(Receive())
	clock.BlockingAdvance(time.Second * 5)
	Eventually(tickChan).Should(Receive())

	Expect(worker.IsDone()).To(BeFalse())
	worker.Stop()
	Expect(worker.IsDone()).To(BeTrue())
	Eventually(errChan).Should(Receive(BeNil()))
}

func (s *WorkerSuite) TestBadInject(t sweet.T) {
	worker := NewWorker(&badInjectWorkerSpec{})
	worker.Services = makeBadContainer()
	worker.Health = nacelle.NewHealth()

	err := worker.Init(testConfig)
	Expect(err).NotTo(BeNil())
	Expect(err.Error()).To(ContainSubstring("ServiceA"))
}

func (s *WorkerSuite) TestInitError(t sweet.T) {
	var (
		spec   = NewMockWorkerSpec()
		worker = makeWorker(spec, glock.NewRealClock())
	)

	spec.InitFunc.SetDefaultHook(func(config nacelle.Config, worker *Worker) error {
		return fmt.Errorf("oops")
	})

	err := worker.Init(testConfig)
	Expect(err).To(MatchError("oops"))
}

func (s *WorkerSuite) TestTickError(t sweet.T) {
	var (
		spec    = NewMockWorkerSpec()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)

	spec.TickFunc.SetDefaultHook(func(ctx context.Context) error {
		return fmt.Errorf("oops")
	})

	err := worker.Init(testConfig)
	Expect(err).To(BeNil())

	go func() {
		errChan <- worker.Start()
	}()

	Eventually(errChan).Should(Receive(MatchError("oops")))
	Expect(worker.IsDone()).To(BeTrue())
}

func (s *WorkerSuite) TestTickContext(t sweet.T) {
	var (
		spec    = NewMockWorkerSpec()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)

	spec.TickFunc.SetDefaultHook(func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	})

	err := worker.Init(testConfig)
	Expect(err).To(BeNil())

	go func() {
		errChan <- worker.Start()
	}()

	worker.Stop()
	Eventually(errChan).Should(Receive(BeNil()))
	Expect(worker.IsDone()).To(BeTrue())
}

func makeWorker(spec WorkerSpec, clock glock.Clock) *Worker {
	worker := newWorker(spec, clock)
	worker.Services = nacelle.NewServiceContainer()
	worker.Health = nacelle.NewHealth()
	return worker
}

//
// Bad Injection

type A struct{ X int }
type B struct{ X float64 }

type badInjectWorkerSpec struct {
	ServiceA *A `service:"A"`
}

func (s *badInjectWorkerSpec) Init(c nacelle.Config, w *Worker) error { return nil }
func (s *badInjectWorkerSpec) Tick(ctx context.Context) error         { return nil }

func makeBadContainer() nacelle.ServiceContainer {
	container := nacelle.NewServiceContainer()
	container.Set("A", &B{})
	return container
}
