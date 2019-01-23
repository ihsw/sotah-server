package state

import (
	"testing"

	"github.com/sotah-inc/server/app/pkg/messenger/codes"

	"github.com/sotah-inc/server/app/pkg/messenger/subjects"

	"github.com/stretchr/testify/assert"
)

func TestListenForRegions(t *testing.T) {
	sta := State{}

	// connecting
	mess, err := newMessengerFromEnvVars("NATS_HOST", "NATS_PORT")
	if !assert.Nil(t, err) {
		return
	}
	sta.Messenger = mess

	// building test status
	reg := region{Hostname: "us.battle.net"}
	s, err := newStatusFromFilepath(reg, "./TestData/Realm-status.json")
	if !assert.Nil(t, err) {
		return
	}
	if !validateStatus(t, reg, s) {
		return
	}
	sta.Statuses = map[regionName]status{reg.Name: s}

	// building test config
	c, err := newConfigFromFilepath("./TestData/config.json")
	if !assert.Nil(t, err) || !assert.NotEmpty(t, c.ClientID) {
		return
	}
	sta.Regions = c.Regions

	// setting up a listener for responding to status requests
	stop := make(chan interface{})
	err = sta.ListenForRegions(stop)
	if !assert.Nil(t, err) {
		return
	}

	// subscribing to receive Regions
	regs, err := newRegionsFromMessenger(mess)
	if !assert.Nil(t, err) || !assert.NotZero(t, len(regs)) {
		stop <- struct{}{}

		return
	}

	// flagging the status listener to exit
	stop <- struct{}{}
}

func TestListenForGenericTestErrors(t *testing.T) {
	sta := State{}

	// connecting
	mess, err := newMessengerFromEnvVars("NATS_HOST", "NATS_PORT")
	if !assert.Nil(t, err) {
		return
	}
	sta.Messenger = mess

	// setting up a listener for responding to status requests
	stop := make(chan interface{})
	err = sta.ListenForGenericTestErrors(stop)
	if !assert.Nil(t, err) {
		return
	}

	// requesting a message from
	msg, err := sta.Messenger.request(subjects.GenericTestErrors, []byte{})
	if !assert.Nil(t, err) {
		stop <- struct{}{}

		return
	}

	// validating the response code
	if !assert.Equal(t, msg.Code, codes.GenericError) {
		return
	}

	// flagging the status listener to exit
	stop <- struct{}{}
}
