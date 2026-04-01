package main

import "github.com/kilo-org/kilo-docker/pkg/services"

func getService(name string) *services.Service {
	return services.GetService(name)
}
