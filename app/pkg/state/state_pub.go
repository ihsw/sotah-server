package state

import (
	"fmt"

	"github.com/sotah-inc/server/app/pkg/bus"
	"github.com/sotah-inc/server/app/pkg/logging"
	"github.com/sotah-inc/server/app/pkg/messenger"
	"github.com/sotah-inc/server/app/pkg/metric"
	"github.com/sotah-inc/server/app/pkg/state/subjects"
	"github.com/twinj/uuid"
)

type PubStateConfig struct {
	GCloudProjectID string

	MessengerHost string
	MessengerPort int
}

func NewPubState(config PubStateConfig) (PubState, error) {
	// establishing an initial state
	tState := PubState{
		State: NewState(uuid.NewV4(), true),
	}

	// connecting to the messenger host
	logging.Info("Connecting messenger")
	mess, err := messenger.NewMessenger(config.MessengerHost, config.MessengerPort)
	if err != nil {
		return PubState{}, err
	}
	tState.IO.Messenger = mess

	// establishing a bus
	bu, err := bus.NewBus(config.GCloudProjectID, "pub")
	tState.IO.Bus = bu

	// initializing a reporter
	tState.IO.Reporter = metric.NewReporter(mess)

	// gathering regions
	logging.Info("Gathering regions")
	regions, err := tState.NewRegions()
	if err != nil {
		return PubState{}, err
	}
	tState.Regions = regions

	// establishing listeners
	tState.Listeners = NewListeners(SubjectListeners{
		subjects.Boot: tState.ListenForBoot,
	})

	return tState, nil
}

type PubState struct {
	State
}

func (tState PubState) ListenForBoot(stop ListenStopChan) error {
	err := tState.IO.Bus.SubscribeToTopic(string(subjects.Boot), stop, func(busMsg bus.Message) {
		logging.WithField("subject", subjects.Boot).Info("Received message")

		msg := bus.NewMessage()
		msg.Data = fmt.Sprintf("Hello, %s!", busMsg.Data)
		if _, err := tState.IO.Bus.ReplyTo(busMsg, msg); err != nil {
			logging.WithField("error", err.Error()).Error("Failed to reply to response message")

			return
		}

		return
	})
	if err != nil {
		return err
	}

	return nil
}