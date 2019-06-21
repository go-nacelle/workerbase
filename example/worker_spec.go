package main

import (
	"context"

	"github.com/go-nacelle/nacelle"
	"github.com/go-nacelle/workerbase"
)

type WorkerSpec struct {
	Logger         nacelle.Logger `service:"logger"`
	WeatherService WeatherService `service:"weather"`
}

func NewWorkerSpec() workerbase.WorkerSpec {
	return &WorkerSpec{}
}

func (ws *WorkerSpec) Init(config nacelle.Config) error {
	return nil
}

func (ws *WorkerSpec) Tick(ctx context.Context) error {
	temp, err := ws.WeatherService.CurrentAverageTemperature(ctx)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil
		default:
			return err
		}
	}

	ws.Logger.Info("Average temp is %.2f", temp)
	return nil
}
