package test2

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/sotah-inc/server/app/pkg/logging"

	"github.com/sotah-inc/server/app/pkg/bus"
	"github.com/sotah-inc/server/app/pkg/store"
)

var projectId = os.Getenv("GCP_PROJECT")
var busClient bus.Client
var storeClient store.Client

func init() {
	var err error
	busClient, err = bus.NewClient(projectId, "fn-test2")
	if err != nil {
		log.Fatalf("Failed to create new bus client: %s", err.Error())

		return
	}

	storeClient, err = store.NewClient(projectId)
	if err != nil {
		log.Fatalf("Failed to create new store client: %s", err.Error())

		return
	}
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

func HelloPubSub(_ context.Context, m PubSubMessage) error {
	var in bus.Message
	if err := json.Unmarshal(m.Data, &in); err != nil {
		return err
	}

	var job bus.LoadRegionRealmTimestampsInJob
	if err := json.Unmarshal([]byte(in.Data), &job); err != nil {
		return err
	}

	logging.WithFields(logrus.Fields{
		"region":      job.RegionName,
		"realm":       job.RealmSlug,
		"target-time": job.TargetTimestamp,
	}).Info("Received request to process auctions")

	//region := sotah.Region{Name: blizzard.RegionName(job.RegionName)}
	//realm := sotah.Realm{
	//	Realm:  blizzard.Realm{Slug: blizzard.RealmSlug(job.RealmSlug)},
	//	Region: region,
	//}
	//targetTime := time.Unix(int64(job.TargetTimestamp), 0)

	//aucs, err := storeClient.GetAuctions(realm, targetTime)
	//if err != nil {
	//	return err
	//}

	return nil
}
