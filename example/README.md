# Worker Base Example

A trivial example application to showcase the [workerbase](https://nacelle.dev/docs/base-procsses/workerbase) library.

## Overview

This example application uses continuously pings the [MetaWeather API](https://www.metaweather.com) to get the average temperature for a configurable location. The **main** function boots [nacelle](https://nacelle.dev/docs/core) with a initializer that creates a weather service and a worker spec for the process provided by this library. The service created by the former is injected into the later.

## Building and Running

If running in Docker, simply run `docker-compose up`. This will compile the example application via a multi-stage build and start a container for the worker.

If running locally, simply build with `go build` (using Go 1.12 or above) and invoke with `WOEID=2451822 WORKER_TICK_INTERVAL=5 ./example`. Change the *Where On Earth ID* to get the weather for another location, and change the tick interval to increase or decrease the rate of requests.
