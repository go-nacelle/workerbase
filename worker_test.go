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
	"github.com/go-nacelle/nacelle/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type WorkerSuite struct{}

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
	worker.Config = testConfig

	ctx := context.Background()
	err := worker.Init(ctx)
	assert.Nil(t, err)

	go func() {
		errChan <- worker.Run(ctx)
	}()

	eventually(t, receiveStruct(tickChan))
	assertStructChanDoesNotReceive(t, tickChan)
	clock.BlockingAdvance(time.Second * 5)
	eventually(t, receiveStruct(tickChan))
	assertStructChanDoesNotReceive(t, tickChan)
	clock.BlockingAdvance(time.Second * 5)
	eventually(t, receiveStruct(tickChan))

	worker.Stop(ctx)
	value := readErrorValue(t, errChan)
	assert.Nil(t, value)
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
	worker.Config = nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
		"worker_tick_interval": "60",
	}))

	ctx := context.Background()
	err := worker.Init(ctx)

	assert.Nil(t, err)

	go func() {
		errChan <- worker.Run(ctx)
	}()

	clock.BlockingAdvance(time.Minute)
	clock.BlockingAdvance(time.Minute)
	clock.BlockingAdvance(time.Minute)
	clock.BlockingAdvance(time.Minute)
	eventually(t, func() bool { return lockedLen() >= 4 })

	worker.Stop(ctx)
	value := readErrorValue(t, errChan)
	assert.Nil(t, value)

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
	worker.Config = nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
		"worker_tick_interval": "60",
		"worker_strict_clock":  "true",
	}))

	ctx := context.Background()
	err := worker.Init(ctx)

	assert.Nil(t, err)

	go func() {
		errChan <- worker.Run(ctx)
	}()

	clock.BlockingAdvance(time.Second * 57)
	clock.BlockingAdvance(time.Second * 55)
	clock.BlockingAdvance(time.Second * 48)
	eventually(t, func() bool { return lockedLen() == 3 })

	worker.Stop(ctx)
	value := readErrorValue(t, errChan)
	assert.Nil(t, value)

	sort.Slice(times[:3], func(i, j int) bool {
		return times[i].Before(times[j])
	})

	expected := []time.Time{
		start,
		start.Add(time.Minute * 1),
		start.Add(time.Minute * 2),
	}
	assert.Equal(t, expected, times[:3])
}

func TestBadInject(t *testing.T) {
	worker := NewWorker(&badInjectWorkerSpec{})
	worker.Services = makeBadContainer()
	worker.Health = nacelle.NewHealth()
	worker.Config = testConfig

	ctx := context.Background()
	err := worker.Init(ctx)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "ServiceA")
}

func TestTagModifiers(t *testing.T) {
	worker := NewWorker(NewMockWorkerSpecFinalizer(), WithTagModifiers(nacelle.NewEnvTagPrefixer("prefix")))
	worker.Services = nacelle.NewServiceContainer()
	worker.Health = nacelle.NewHealth()
	worker.Config = nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
		"prefix_worker_tick_interval": "3600",
	}))

	ctx := context.Background()
	err := worker.Init(ctx)

	assert.Nil(t, err)
	assert.Equal(t, time.Hour, worker.tickInterval)
}

func TestInitConfig(t *testing.T) {
	var (
		spec   = NewMockWorkerSpecFinalizer()
		clock  = glock.NewMockClock()
		worker = makeWorker(spec, clock)
	)

	worker.Config = nacelle.NewConfig(nacelle.NewTestEnvSourcer(map[string]string{
		"worker_tick_interval": "60",
		"worker_strict_clock":  "true",
	}))

	ctx := context.Background()
	err := worker.Init(ctx)
	require.Nil(t, err)

	assert.True(t, worker.strictClock)
	assert.Equal(t, 60*time.Second, worker.tickInterval)
}

func TestInitError(t *testing.T) {
	var (
		spec   = NewMockWorkerSpecFinalizer()
		worker = makeWorker(spec, glock.NewRealClock())
	)

	spec.InitFunc.SetDefaultHook(func(ctx context.Context) error {
		return fmt.Errorf("oops")
	})
	worker.Config = testConfig

	ctx := context.Background()
	err := worker.Init(ctx)
	assert.EqualError(t, err, "oops")
}

func TestFinalize(t *testing.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)
	worker.Config = testConfig

	ctx := context.Background()
	err := worker.Init(ctx)
	assert.Nil(t, err)

	go func() {
		errChan <- worker.Run(ctx)
	}()

	worker.Stop(ctx)
	value := readErrorValue(t, errChan)
	assert.Nil(t, value)
	mockassert.CalledOnce(t, spec.FinalizeFunc)
}

func TestFinalizeError(t *testing.T) {
	var (
		spec    = NewMockWorkerSpecFinalizer()
		clock   = glock.NewMockClock()
		worker  = makeWorker(spec, clock)
		errChan = make(chan error)
	)

	spec.FinalizeFunc.SetDefaultHook(func(ctx context.Context) error {
		return fmt.Errorf("oops")
	})
	worker.Config = testConfig

	ctx := context.Background()
	err := worker.Init(ctx)
	assert.Nil(t, err)

	go func() {
		errChan <- worker.Run(ctx)
	}()

	worker.Stop(ctx)
	value := readErrorValue(t, errChan)
	assert.EqualError(t, value, "oops")
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

	spec.FinalizeFunc.SetDefaultHook(func(ctx context.Context) error {
		return fmt.Errorf("unheard oops")
	})
	worker.Config = testConfig

	ctx := context.Background()
	err := worker.Init(ctx)
	assert.Nil(t, err)

	go func() {
		errChan <- worker.Run(ctx)
	}()

	worker.Stop(ctx)
	value := readErrorValue(t, errChan)
	assert.EqualError(t, value, "oops")
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
	worker.Config = testConfig

	ctx := context.Background()
	err := worker.Init(ctx)
	assert.Nil(t, err)

	go func() {
		errChan <- worker.Run(ctx)
	}()

	value := readErrorValue(t, errChan)
	assert.EqualError(t, value, "oops")
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
	worker.Config = testConfig

	ctx := context.Background()
	err := worker.Init(ctx)
	assert.Nil(t, err)

	go func() {
		errChan <- worker.Run(ctx)
	}()

	worker.Stop(ctx)
	value := readErrorValue(t, errChan)
	assert.Nil(t, value)
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

func (s *badInjectWorkerSpec) Init(ctx context.Context) error { return nil }
func (s *badInjectWorkerSpec) Tick(ctx context.Context) error { return nil }

func makeBadContainer() *nacelle.ServiceContainer {
	container := nacelle.NewServiceContainer()
	container.Set("A", &B{})
	return container
}
