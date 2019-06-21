package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type WeatherService interface {
	CurrentAverageTemperature(ctx context.Context) (float64, error)
}

type weatherService struct {
	url string
}

type jsonPayload struct {
	Readings []jsonReading `json:"consolidated_weather"`
}

type jsonReading struct {
	Temp float64 `json:"the_temp"`
}

func NewWeatherService(url string) WeatherService {
	return &weatherService{
		url: url,
	}
}

func (ws *weatherService) CurrentAverageTemperature(ctx context.Context) (float64, error) {
	req, err := http.NewRequest("GET", ws.url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	payload := &jsonPayload{}
	if err := json.Unmarshal(content, payload); err != nil {
		return 0, err
	}
	return getAverage(payload.Readings), nil
}

func getAverage(readings []jsonReading) float64 {
	if len(readings) == 0 {
		return 0
	}

	sum := float64(0)
	for _, reading := range readings {
		sum += reading.Temp
	}

	return sum / float64(len(readings))
}
