package main

import (
	"fmt"

	"github.com/go-nacelle/nacelle"
)

type WeatherServiceInitializer struct {
	Services nacelle.ServiceContainer `service:"services"`
}

type Config struct {
	WhereOnEarthID string `env:"woeid" required:"true"`
}

const URLFormat = "https://www.metaweather.com/api/location/%s/"

func NewWeatherServiceInitializer() nacelle.Initializer {
	return &WeatherServiceInitializer{}
}

func (wsi *WeatherServiceInitializer) Init(config nacelle.Config) error {
	weatherConfig := &Config{}
	if err := config.Load(weatherConfig); err != nil {
		return err
	}

	return wsi.Services.Set("weather", NewWeatherService(fmt.Sprintf(URLFormat, weatherConfig.WhereOnEarthID)))
}
