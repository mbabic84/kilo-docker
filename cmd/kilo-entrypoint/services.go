package main

import "github.com/mbabic84/kilo-docker/pkg/services"

func getService(name string) *services.Service {
	return services.GetService(name)
}
