package main

import (
	"github.com/go-nacelle/nacelle"
	"github.com/go-nacelle/workerbase"
)

func setup(processes nacelle.ProcessContainer, services nacelle.ServiceContainer) error {
	processes.RegisterInitializer(NewWeatherServiceInitializer(), nacelle.WithInitializerName("weather"))
	processes.RegisterProcess(workerbase.NewWorker(NewWorkerSpec()), nacelle.WithProcessName("worker"))
	return nil
}

func main() {
	nacelle.NewBootstrapper("workerbase-example", setup).BootAndExit()
}
