package sync_all_items

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/sotah-inc/server/app/pkg/blizzard"
	"github.com/sotah-inc/server/app/pkg/bus"
	"github.com/sotah-inc/server/app/pkg/bus/codes"
	"github.com/sotah-inc/server/app/pkg/database"
	"github.com/sotah-inc/server/app/pkg/logging"
	"github.com/sotah-inc/server/app/pkg/metric"
	"github.com/sotah-inc/server/app/pkg/sotah"
	"github.com/sotah-inc/server/app/pkg/state/subjects"
)

var (
	projectId = os.Getenv("GCP_PROJECT")

	busClient                bus.Client
	syncItemsTopic           *pubsub.Topic
	filterInItemsToSyncTopic *pubsub.Topic
)

func init() {
	var err error
	busClient, err = bus.NewClient(projectId, "fn-sync-all-items")
	if err != nil {
		log.Fatalf("Failed to create new bus client: %s", err.Error())

		return
	}
	syncItemsTopic, err = busClient.FirmTopic(string(subjects.SyncItems))
	if err != nil {
		log.Fatalf("Failed to get firm topic: %s", err.Error())

		return
	}
	filterInItemsToSyncTopic, err = busClient.FirmTopic(string(subjects.FilterInItemsToSync))
	if err != nil {
		log.Fatalf("Failed to get firm topic: %s", err.Error())

		return
	}
}

func HandleItemIds(syncPayload database.ItemsSyncPayload) error {
	if len(syncPayload.Ids) == 0 {
		logging.Info("No item-ids in sync-payload, skipping")

		return nil
	}

	// batching items together
	logging.WithField("ids", len(syncPayload.Ids)).Info("Batching ids together")
	itemIdsBatches := sotah.NewItemIdsBatches(syncPayload.Ids, 1000)

	// producing messages
	logging.WithField("batches", len(itemIdsBatches)).Info("Producing messages for enqueueing")
	messages, err := bus.NewItemBatchesMessages(itemIdsBatches)
	if err != nil {
		return err
	}

	// enqueueing them
	logging.WithField("messages", len(messages)).Info("Bulk-requesting with messages")
	responses, err := busClient.BulkRequest(syncItemsTopic, messages, 60*time.Second)
	if err != nil {
		return err
	}

	// going over the responses
	logging.WithField("responses", len(responses)).Info("Going over responses")
	for _, msg := range responses {
		if msg.Code != codes.Ok {
			logging.WithField("error", msg.Err).Error("Request from sync-items failed")

			continue
		}

		logging.WithField("batch", msg.ReplyToId).Info("Finished batch")
	}

	return nil
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

func SyncAllItems(_ context.Context, m PubSubMessage) error {
	var in bus.Message
	if err := json.Unmarshal(m.Data, &in); err != nil {
		return err
	}

	// validating that the provided item-ids are valid
	providedItemIds, err := blizzard.NewItemIds(in.Data)
	if err != nil {
		return err
	}
	encodedItemIds, err := providedItemIds.EncodeForDelivery()
	if err != nil {
		return err
	}

	startTime := time.Now()

	// filtering in items-to-sync
	response, err := busClient.Request(filterInItemsToSyncTopic, encodedItemIds, 30*time.Second)
	if err != nil {
		return err
	}

	// optionally halting
	if response.Code != codes.Ok {
		return errors.New("response code was not ok")
	}

	// parsing response data
	syncPayload, err := database.NewItemsSyncPayload(response.Data)
	if err != nil {
		return err
	}

	// handling item-ids
	if err := HandleItemIds(syncPayload); err != nil {
		return err
	}

	// reporting metrics
	if err := busClient.PublishMetrics(metric.Metrics{
		"sync_all_items_duration": int(int64(time.Now().Sub(startTime)) / 1000 / 1000 / 1000),
	}); err != nil {
		return err
	}

	return nil
}
