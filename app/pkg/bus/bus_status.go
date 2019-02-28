package bus

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/sotah-inc/server/app/pkg/blizzard"
	"github.com/sotah-inc/server/app/pkg/bus/codes"
	"github.com/sotah-inc/server/app/pkg/sotah"
	"github.com/sotah-inc/server/app/pkg/state"
	"github.com/sotah-inc/server/app/pkg/state/subjects"
)

func (c Client) NewStatus(reg sotah.Region) (sotah.Status, error) {
	lm := state.StatusRequest{RegionName: reg.Name}
	encodedMessage, err := json.Marshal(lm)
	if err != nil {
		return sotah.Status{}, err
	}

	msg, err := c.RequestFromTopic(string(subjects.Status), string(encodedMessage), 5*time.Second)
	if err != nil {
		return sotah.Status{}, err
	}

	if msg.Code != codes.Ok {
		return sotah.Status{}, errors.New(msg.Err)
	}

	stat, err := blizzard.NewStatus([]byte(msg.Data))
	if err != nil {
		return sotah.Status{}, err
	}

	return sotah.NewStatus(reg, stat), nil
}
