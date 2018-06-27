package main

import (
	"testing"

	"github.com/ihsw/sotah-server/app/blizzard"
	"github.com/ihsw/sotah-server/app/utiltest"
	"github.com/stretchr/testify/assert"
)

func validateStatus(t *testing.T, reg region, s status) bool {
	if !assert.NotEmpty(t, s.Realms) {
		return false
	}

	for _, rea := range s.Realms {
		if !assert.Equal(t, reg.Hostname, rea.region.Hostname) {
			return false
		}
	}

	return true
}

func TestNewStatusFromFilepath(t *testing.T) {
	reg := region{Hostname: "us.battle.net"}
	s, err := newStatusFromFilepath(reg, "./TestData/realm-status.json")
	if !assert.Nil(t, err) {
		return
	}
	if !validateStatus(t, reg, s) {
		return
	}
}

func TestNewStatusFromMessenger(t *testing.T) {
	sta := state{}

	// connecting
	mess, err := newMessengerFromEnvVars("NATS_HOST", "NATS_PORT")
	if !assert.Nil(t, err) {
		return
	}
	sta.messenger = mess

	// building test status
	reg := region{Name: "us", Hostname: "us.battle.net"}
	s, err := newStatusFromFilepath(reg, "./TestData/realm-status.json")
	if !assert.Nil(t, err) {
		return
	}
	if !validateStatus(t, reg, s) {
		return
	}
	sta.statuses = map[regionName]status{reg.Name: s}
	sta.regions = []region{reg}

	// setting up a subscriber that will publish status retrieval requests
	stop := make(chan interface{})
	err = sta.listenForStatus(stop)
	if !assert.Nil(t, err) {
		return
	}

	// subscribing to receive statuses
	receivedStatus, err := newStatusFromMessenger(reg, mess)
	if !assert.Nil(t, err) || !assert.Equal(t, s.region.Hostname, receivedStatus.region.Hostname) {
		stop <- struct{}{}
		return
	}

	// flagging the status listener to exit
	stop <- struct{}{}
}
func TestNewStatus(t *testing.T) {
	body, err := utiltest.ReadFile("./TestData/realm-status.json")
	if !assert.Nil(t, err) {
		return
	}

	blizzStatus, err := blizzard.NewStatus(body)
	if !assert.Nil(t, err) {
		return
	}

	reg := region{Hostname: "us.battle.net"}
	s := newStatus(reg, blizzStatus)
	if !validateStatus(t, reg, s) {
		return
	}
}
