package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-nacelle/nacelle"
	"github.com/go-nacelle/workerbase"
)

type WorkerSpec struct {
	Logger nacelle.Logger `service:"logger"`
	worker *workerbase.Worker
	url    string
}

type WorkerConfig struct {
	WhereOnEarthID string `env:"woeid" default:"2451822"`
}

func (ws *WorkerSpec) Init(config nacelle.Config, worker *workerbase.Worker) error {
	workerConfig := &WorkerConfig{}
	if err := config.Load(workerConfig); err != nil {
		return err
	}

	ws.worker = worker
	ws.url = fmt.Sprintf("https://www.metaweather.com/api/location/%s/", workerConfig.WhereOnEarthID)
	return nil
}

func (ws *WorkerSpec) Tick() error {
	req, err := http.NewRequest("GET", ws.url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	payload := &struct {
		Readings []struct {
			Temp float64 `json:"the_temp"`
		} `json:"consolidated_weather"`
	}{}

	if err := json.Unmarshal(content, payload); err != nil {
		return err
	}

	sum := float64(0)
	for _, reading := range payload.Readings {
		sum += reading.Temp
	}

	if len(payload.Readings) == 0 {
		ws.Logger.Info("No readings.")
		return nil
	}

	ws.Logger.Info("Average temp is %.2f", sum/float64(len(payload.Readings)))
	return nil
}

func setup(processes nacelle.ProcessContainer, services nacelle.ServiceContainer) error {
	processes.RegisterProcess(workerbase.NewWorker(&WorkerSpec{}))
	return nil
}

func main() {
	nacelle.NewBootstrapper("workerbase-example", setup).BootAndExit()
}
