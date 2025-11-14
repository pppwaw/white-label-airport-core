package service_manager

import (
	"github.com/sagernet/sing-box/adapter"
)

var (
	services    = []adapter.Service{}
	preservices = []adapter.Service{}
)

func RegisterPreservice(service adapter.Service) {
	preservices = append(preservices, service)
}

func Register(service adapter.Service) {
	services = append(services, service)
}

func StartServices() error {
	if err := CloseServices(); err != nil {
		return err
	}
	if err := startServiceList(preservices); err != nil {
		return err
	}
	if err := startServiceList(services); err != nil {
		return err
	}
	return nil
}

func CloseServices() error {
	for _, service := range services {
		if err := service.Close(); err != nil {
			return err
		}
	}
	for _, service := range preservices {
		if err := service.Close(); err != nil {
			return err
		}
	}
	return nil
}

func startServiceList(list []adapter.Service) error {
	for _, stage := range adapter.ListStartStages {
		for _, service := range list {
			if err := service.Start(stage); err != nil {
				return err
			}
		}
	}
	return nil
}
