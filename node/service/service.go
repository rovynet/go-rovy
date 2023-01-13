package rservice

import (
	"errors"
	"log"
)

type Service interface {
	Start() error
	Stop() error
}

var ErrServiceUnknown = errors.New("service unknown")
var ErrServiceRunning = errors.New("service already running")
var ErrServiceNotRunning = errors.New("service not running")

type ServiceManager struct {
	services map[string]Service
	logger   *log.Logger
}

func NewServiceManager(logger *log.Logger) *ServiceManager {
	sm := &ServiceManager{
		services: make(map[string]Service),
		logger:   logger,
	}
	return sm
}

func (sm *ServiceManager) Add(tag string, svc Service) error {
	svc2, ok := sm.services[tag]
	if ok {
		svc2.Stop()
	}
	sm.services[tag] = svc
	return nil
}

func (sm *ServiceManager) Start(tag string) error {
	svc, ok := sm.services[tag]
	if !ok {
		return ErrServiceUnknown
	}
	return svc.Start()
}

func (sm *ServiceManager) Stop(tag string) error {
	svc, ok := sm.services[tag]
	if !ok {
		return ErrServiceUnknown
	}
	return svc.Stop()
}
