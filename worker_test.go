package workerbase

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/derision-test/glock"
	mockassert "github.com/derision-test/go-mockgen/testutil/assert"
	"github.com/go-nacelle/nacelle"
	"github.com/stretchr/testify/require"
)

var testConfig = nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
	"worker_tick_interval": "5",
}))

func TestRunAndStop(t *testing.T) {
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
	require.Nil(t, err)

	go func() {
		errChan <- worker.Start()
	}()

	eventually(t, receiveStruct(tickChan))
	assertStructChanDoesNotReceive(t, tickChan)
	clock.BlockingAdvance(time.Second * 5)
	eventually(t, receiveStruct(tickChan))
	assertStructChanDoesNotReceive(t, tickChan)
	clock.BlockingAdvance(time.Second * 5)
	eventually(t, receiveStruct(tickChan))

	worker.Stop()
	value := readErrorValue(t, errChan)
	require.Nil(t, value)
}

func TestNonStrict(t *testing.T) {
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

	require.Nil(t, err)

	go func() {
		errChan <- worker.Start()
	}()

	clock.BlockingAdvance(time.Minute)
	clock.BlockingAdvance(time.Minute)
	clock.BlockingAdvance(time.Minute)
	clock.BlockingAdvance(time.Minute)
	eventually(t, func() bool { return lockedLen() >= 4 })

	worker.Stop()
	value := readErrorValue(t, errChan)
	require.Nil(t, value)

	sort.Slice(times[:4], func(i, j int) bool {
		return times[i].Before(times[j])
	})

	expected := []time.Time{
		start,
		start.Add(time.Minute * 1).Add(time.Second * 30 * 1),
		start.Add(time.Minute * 2).Add(time.Second * 30 * 2),
		start.Add(time.Minute * 3).Add(time.Second * 30 * 3),
	}
	require.Equal(t, expected, times[:4])
}

func TestStrict(t *testing.T) {
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

	require.Nil(t, err)

	go func() {
		errChan <- worker.Start()
	}()

	clock.BlockingAdvance(time.Second * 57)
	clock.BlockingAdvance(time.Second * 55)
	clock.BlockingAdvance(time.Second * 48)
	eventually(t, func() bool { return lockedLen() == 3 })

	worker.Stop()
	value := readErrorValue(t, errChan)
	require.Nil(t, value)

	sort.Slice(times[:3], func(i, j int) bool {
		return times[i].Before(times[j])
	})

	expected := []time.Time{
		start,
		start.Add(time.Minute * 1),
		start.Add(time.Minute * 2),
	}
	require.Equal(t, expected, times[:3])
}

func TestBadInject(t *testing.T) {
	worker := NewWorker(&badInjectWorkerSpec{})
	worker.Services = makeBadContainer()
	worker.Health = nacelle.NewHealth()

	err := worker.Init(testConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "ServiceA")
}

func TestTagModifiers(t *testing.T) {
	worker := NewWorker(NewMockWorkerSpecFinalizer(), WithTagModifiers(nacelle.NewEnvTagPrefixer("prefix")))
	worker.Services = nacelle.NewServiceContainer()
	worker.Health = nacelle.NewHealth()

	err := worker.Init(nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
		"prefix_worker_tick_interval": "3600",
	})))

	require.Nil(t, err)
	require.Equal(t, time.Hour, worker.tickInterval)
}

func TestInitError(t *testing.T) {
	var (
		spec   = NewMockWorkerSpecFinalizer()
		worker = makeWorker(spec, glock.NewRealClock())
	)

	spec.InitFunc.SetDefaultHook(func(config nacelle.Config) error {
		return fmt.Errorf("oops")
	})

	err := worker.Init(testConfig)
	require.EqualError(t, err, "oops")
}

func TestFinalize(t *testing.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)

	err := worker.Init(testConfig)
	require.Nil(t, err)

	go func() {
		errChan <- worker.Start()
	}()

	worker.Stop()
	value := readErrorValue(t, errChan)
	require.Nil(t, value)
	mockassert.CalledOnce(t, spec.FinalizeFunc)
}

func TestFinalizeError(t *testing.T) {
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
	require.Nil(t, err)

	go func() {
		errChan <- worker.Start()
	}()

	worker.Stop()
	value := readErrorValue(t, errChan)
	require.EqualError(t, value, "oops")
	mockassert.CalledOnce(t, spec.FinalizeFunc)
}

func TestFinalizeErrorDoesNotOverwrite(t *testing.T) {
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
	require.Nil(t, err)

	go func() {
		errChan <- worker.Start()
	}()

	worker.Stop()
	value := readErrorValue(t, errChan)
	require.EqualError(t, value, "oops")
	mockassert.CalledOnce(t, spec.FinalizeFunc)
}

func TestTickError(t *testing.T) {
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
	require.Nil(t, err)

	go func() {
		errChan <- worker.Start()
	}()

	value := readErrorValue(t, errChan)
	require.EqualError(t, value, "oops")
}

func TestTickContext(t *testing.T) {
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
	require.Nil(t, err)

	go func() {
		errChan <- worker.Start()
	}()

	worker.Stop()
	value := readErrorValue(t, errChan)
	require.Nil(t, value)
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
