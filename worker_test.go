package workerbase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aphistic/sweet"
	"github.com/efritz/glock"
	. "github.com/efritz/go-mockgen/matchers"
	"github.com/go-nacelle/nacelle"
	. "github.com/onsi/gomega"
)

type WorkerSuite struct{}

var testConfig = nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
	"worker_tick_interval": "5",
}))

func (s *WorkerSuite) TestRunAndStop(t sweet.T) {
	var (
		spec     = NewMockWorkerSpecFinalizer()
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

	worker.Stop()
	Eventually(errChan).Should(Receive(BeNil()))
}

func (s *WorkerSuite) TestNonStrict(t sweet.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)

	times := []time.Time{}
	mutex := sync.Mutex{}

	lockedLen := func() int {
		mutex.Lock()
		defer mutex.Unlock()
		return len(times)
	}

	start := time.Now()
	clock.SetCurrent(start)

	spec.TickFunc.SetDefaultHook(func(ctx context.Context) error {
		mutex.Lock()
		times = append(times, clock.Now())
		mutex.Unlock()

		clock.Advance(time.Second * 30)
		return nil
	})

	err := worker.Init(nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
		"worker_tick_interval": "60",
	})))

	Expect(err).To(BeNil())

	go func() {
		errChan <- worker.Start()
	}()

	clock.BlockingAdvance(time.Minute)
	clock.BlockingAdvance(time.Minute)
	clock.BlockingAdvance(time.Minute)
	clock.BlockingAdvance(time.Minute)
	Eventually(lockedLen).Should(BeNumerically(">=", 4))

	worker.Stop()
	Eventually(errChan).Should(Receive(BeNil()))
	Expect(times[:4]).To(ConsistOf(
		start,
		start.Add(time.Minute*1).Add(time.Second*30*1),
		start.Add(time.Minute*2).Add(time.Second*30*2),
		start.Add(time.Minute*3).Add(time.Second*30*3),
	))
}

func (s *WorkerSuite) TestStrict(t sweet.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)

	worker.tickInterval = time.Minute
	worker.strictClock = true

	times := []time.Time{}
	mutex := sync.Mutex{}

	lockedLen := func() int {
		mutex.Lock()
		defer mutex.Unlock()
		return len(times)
	}

	durations := []time.Duration{
		time.Second * 3,
		time.Second * 5,
		time.Second * 12,
	}

	start := time.Now()
	clock.SetCurrent(start)

	spec.TickFunc.SetDefaultHook(func(ctx context.Context) error {
		if len(durations) == 0 {
			<-ctx.Done()
			return nil
		}

		mutex.Lock()
		times = append(times, clock.Now())
		mutex.Unlock()

		d := durations[0]
		durations = durations[1:]
		clock.Advance(d)
		return nil
	})

	err := worker.Init(nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
		"worker_tick_interval": "60",
		"worker_strict_clock":  "true",
	})))

	Expect(err).To(BeNil())

	go func() {
		errChan <- worker.Start()
	}()

	clock.BlockingAdvance(time.Second * 57)
	clock.BlockingAdvance(time.Second * 55)
	clock.BlockingAdvance(time.Second * 48)
	Eventually(lockedLen).Should(Equal(3))

	worker.Stop()
	Eventually(errChan).Should(Receive(BeNil()))
	Expect(times[:3]).To(ConsistOf(
		start,
		start.Add(time.Minute*1),
		start.Add(time.Minute*2),
	))
}

func (s *WorkerSuite) TestBadInject(t sweet.T) {
	worker := NewWorker(&badInjectWorkerSpec{})
	worker.Services = makeBadContainer()
	worker.Health = nacelle.NewHealth()

	err := worker.Init(testConfig)
	Expect(err).NotTo(BeNil())
	Expect(err.Error()).To(ContainSubstring("ServiceA"))
}

func (s *WorkerSuite) TestTagModifiers(t sweet.T) {
	worker := NewWorker(NewMockWorkerSpecFinalizer(), WithTagModifiers(nacelle.NewEnvTagPrefixer("prefix")))
	worker.Services = nacelle.NewServiceContainer()
	worker.Health = nacelle.NewHealth()

	err := worker.Init(nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
		"prefix_worker_tick_interval": "3600",
	})))

	Expect(err).To(BeNil())
	Expect(worker.tickInterval).To(Equal(time.Hour))
}

func (s *WorkerSuite) TestInitError(t sweet.T) {
	var (
		spec   = NewMockWorkerSpecFinalizer()
		worker = makeWorker(spec, glock.NewRealClock())
	)

	spec.InitFunc.SetDefaultHook(func(config nacelle.Config) error {
		return fmt.Errorf("oops")
	})

	err := worker.Init(testConfig)
	Expect(err).To(MatchError("oops"))
}

func (s *WorkerSuite) TestFinalize(t sweet.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)

	err := worker.Init(testConfig)
	Expect(err).To(BeNil())

	go func() {
		errChan <- worker.Start()
	}()

	worker.Stop()
	Eventually(errChan).Should(Receive(BeNil()))
	Expect(spec.FinalizeFunc).To(BeCalledOnce())
}

func (s *WorkerSuite) TestFinalizeError(t sweet.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)

	spec.FinalizeFunc.SetDefaultHook(func() error {
		return fmt.Errorf("oops")
	})

	err := worker.Init(testConfig)
	Expect(err).To(BeNil())

	go func() {
		errChan <- worker.Start()
	}()

	worker.Stop()
	Eventually(errChan).Should(Receive(MatchError("oops")))
	Expect(spec.FinalizeFunc).To(BeCalledOnce())
}

func (s *WorkerSuite) TestFinalizeErrorDoesNotOverwrite(t sweet.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)

	spec.TickFunc.SetDefaultHook(func(ctx context.Context) error {
		return fmt.Errorf("oops")
	})

	spec.FinalizeFunc.SetDefaultHook(func() error {
		return fmt.Errorf("unheard oops")
	})

	err := worker.Init(testConfig)
	Expect(err).To(BeNil())

	go func() {
		errChan <- worker.Start()
	}()

	worker.Stop()
	Eventually(errChan).Should(Receive(MatchError("oops")))
	Expect(spec.FinalizeFunc).To(BeCalledOnce())
}

func (s *WorkerSuite) TestTickError(t sweet.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
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
}

func (s *WorkerSuite) TestTickContext(t sweet.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
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

func (s *badInjectWorkerSpec) Init(c nacelle.Config) error    { return nil }
func (s *badInjectWorkerSpec) Tick(ctx context.Context) error { return nil }

func makeBadContainer() nacelle.ServiceContainer {
	container := nacelle.NewServiceContainer()
	container.Set("A", &B{})
	return container
}
