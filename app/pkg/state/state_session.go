package state

import (
	"encoding/json"

	nats "github.com/nats-io/go-nats"
	"github.com/sotah-inc/server/app/pkg/messenger"
	"github.com/sotah-inc/server/app/pkg/messenger/codes"
	"github.com/sotah-inc/server/app/pkg/messenger/subjects"
)

type sessionSecretData struct {
	SessionSecret string `json:"session_secret"`
}

func (sta State) ListenForSessionSecret(stop ListenStopChan) error {
	err := sta.Messenger.Subscribe(subjects.SessionSecret, stop, func(natsMsg nats.Msg) {
		m := messenger.NewMessage()

		encodedData, err := json.Marshal(sessionSecretData{sta.SessionSecret.String()})
		if err != nil {
			m.Err = err.Error()
			m.Code = codes.GenericError
			sta.Messenger.ReplyTo(natsMsg, m)

			return
		}

		m.Data = string(encodedData)
		sta.Messenger.ReplyTo(natsMsg, m)
	})
	if err != nil {
		return err
	}

	return nil
}
