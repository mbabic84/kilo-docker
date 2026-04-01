package main

import "github.com/mbabic84/kilo-docker/pkg/services"

func getService(name string) *services.Service {
	return services.GetService(name)
}

func isServiceEnabled(cfg config, name string) bool {
	for _, svc := range cfg.enabledServices {
		if svc == name {
			return true
		}
	}
	return false
}
