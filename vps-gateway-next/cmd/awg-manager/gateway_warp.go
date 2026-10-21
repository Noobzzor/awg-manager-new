package main

import (
	"errors"
	gatewaywarp "github.com/hoaxisr/awg-manager/internal/gateway/warp"
)

func (a *app) setupGatewayWARP() (*gatewaywarp.Service, error) {
	if a.sbOrch == nil {
		return nil, errors.New("WARP requires gateway orchestrator")
	}
	service := gatewaywarp.NewService(gatewaywarp.NewStore(a.dataDir), a.sbOrch)
	if err := service.Sync(); err != nil {
		return nil, err
	}
	return service, nil
}
