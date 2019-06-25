package workerbase

import "time"

type Config struct {
	StrictClock           bool `env:"worker_strict_clock"`
	RawWorkerTickInterval int  `env:"worker_tick_interval" default:"0"`

	WorkerTickInterval time.Duration
}

func (c *Config) PostLoad() error {
	c.WorkerTickInterval = time.Duration(c.RawWorkerTickInterval) * time.Second
	return nil
}
