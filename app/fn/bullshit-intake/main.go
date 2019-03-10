package bullshit_intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"github.com/sotah-inc/server/app/pkg/blizzard"
	"github.com/sotah-inc/server/app/pkg/bus"
	"github.com/sotah-inc/server/app/pkg/logging"
	"github.com/sotah-inc/server/app/pkg/sotah"
	"github.com/sotah-inc/server/app/pkg/state/subjects"
	"github.com/sotah-inc/server/app/pkg/store"
	"google.golang.org/api/iterator"
)

var projectId = os.Getenv("GCP_PROJECT")

var storeClient store.Client

var auctionManifestStoreBaseV2 store.AuctionManifestBaseV2
var manifestBucket *storage.BucketHandle

var auctionManifestStoreBaseInter store.AuctionManifestBaseInter
var manifestInterBucket *storage.BucketHandle

var auctionsStoreBaseV2 store.AuctionsBaseV2
var rawAuctionsBucket *storage.BucketHandle

var auctionsStoreBaseInter store.AuctionsBaseInter
var rawAuctionsInterBucket *storage.BucketHandle

var busClient bus.Client
var auctionsCleanupTopic *pubsub.Topic

func init() {
	var err error

	busClient, err = bus.NewClient(projectId, "fn-cleanup-all-expired-manifests")
	if err != nil {
		log.Fatalf("Failed to create new bus client: %s", err.Error())

		return
	}
	auctionsCleanupTopic, err = busClient.FirmTopic(string(subjects.CleanupExpiredManifest))
	if err != nil {
		log.Fatalf("Failed to get firm topic: %s", err.Error())

		return
	}

	storeClient, err = store.NewClient(projectId)
	if err != nil {
		log.Fatalf("Failed to create new store client: %s", err.Error())

		return
	}

	auctionsStoreBaseV2 = store.NewAuctionsBaseV2(storeClient, "us-east1")
	rawAuctionsBucket, err = auctionsStoreBaseV2.GetFirmBucket()
	if err != nil {
		log.Fatalf("Failed to get new manifest bucket: %s", err.Error())

		return
	}

	auctionsStoreBaseInter = store.NewAuctionsBaseInter(storeClient, "us-central1")
	rawAuctionsInterBucket, err = auctionsStoreBaseInter.GetFirmBucket()
	if err != nil {
		log.Fatalf("Failed to get new manifest bucket: %s", err.Error())

		return
	}

	auctionManifestStoreBaseV2 = store.NewAuctionManifestBaseV2(storeClient, "us-east1")
	manifestBucket, err = auctionManifestStoreBaseV2.GetFirmBucket()
	if err != nil {
		log.Fatalf("Failed to get new manifest bucket: %s", err.Error())

		return
	}

	auctionManifestStoreBaseInter = store.NewAuctionManifestBaseInter(storeClient, "us-central1")
	manifestInterBucket, err = auctionManifestStoreBaseInter.GetFirmBucket()
	if err != nil {
		log.Fatalf("Failed to get new manifest bucket: %s", err.Error())

		return
	}
}

func RebuildManifest(realm sotah.Realm) error {
	currentNormalizedTime := sotah.NormalizeTargetDate(time.Now())

	manifests := map[sotah.UnixTimestamp]sotah.AuctionManifest{}

	it := rawAuctionsBucket.Objects(
		storeClient.Context,
		&storage.Query{
			Delimiter: "/",
			Prefix:    fmt.Sprintf("%s/%s/", realm.Region.Name, realm.Slug),
		},
	)
	for {
		objAttrs, err := it.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}

			if err != nil {
				return err
			}
		}

		parts := strings.Split(objAttrs.Name, ".")
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return err
		}

		objTimestamp, err := strconv.Atoi(parts[0])
		if err != nil {
			return err
		}

		normalizedTime := sotah.NormalizeTargetDate(time.Unix(int64(objTimestamp), 0))

		if normalizedTime.After(currentNormalizedTime) {
			continue
		}

		normalizedTimestamp := sotah.UnixTimestamp(normalizedTime.Unix())
		nextManifest := func() sotah.AuctionManifest {
			result, ok := manifests[normalizedTimestamp]
			if !ok {
				return sotah.AuctionManifest{}
			}

			return result
		}()
		manifests[normalizedTimestamp] = append(nextManifest, sotah.UnixTimestamp(objTimestamp))
	}

	for writeAllOutJob := range auctionManifestStoreBaseV2.WriteAll(manifestBucket, realm, manifests) {
		if writeAllOutJob.Err != nil {
			return writeAllOutJob.Err
		}
	}

	return nil
}

func CheckManifestForExpired(realm sotah.Realm) error {
	limit := sotah.NormalizeTargetDate(time.Now()).AddDate(0, 0, -14)
	prefix := fmt.Sprintf("%s/%s/", realm.Region.Name, realm.Slug)
	it := manifestBucket.Objects(storeClient.Context, &storage.Query{Prefix: prefix})
	for {
		objAttrs, err := it.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}

			if err != nil {
				return err
			}
		}

		targetTimestamp, err := strconv.Atoi(objAttrs.Name[len(prefix):(len(objAttrs.Name) - len(".json"))])
		if err != nil {
			return err
		}

		targetTime := time.Unix(int64(targetTimestamp), 0)
		if targetTime.After(limit) {
			continue
		}

		logging.WithFields(logrus.Fields{
			"region":           realm.Region.Name,
			"realm":            realm.Slug,
			"target-timestamp": targetTimestamp,
		}).Info("Found expired, enqueueing")

		jsonEncoded, err := json.Marshal(bus.CleanupAuctionManifestJob{
			RegionName:      string(realm.Region.Name),
			RealmSlug:       string(realm.Slug),
			TargetTimestamp: int(targetTime.Unix()),
		})
		if err != nil {
			return err
		}

		msg := bus.NewMessage()
		msg.Data = string(jsonEncoded)

		if _, err := busClient.Publish(auctionsCleanupTopic, msg); err != nil {
			return err
		}
	}

	return nil
}

func TransferBuckets(realm sotah.Realm) error {
	prefix := fmt.Sprintf("%s/%s/", realm.Region.Name, realm.Slug)
	it := manifestBucket.Objects(storeClient.Context, &storage.Query{Prefix: prefix})
	for {
		objAttrs, err := it.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}

			if err != nil {
				return err
			}
		}

		manifest, err := auctionManifestStoreBaseV2.NewAuctionManifest(manifestBucket.Object(objAttrs.Name))
		if err != nil {
			return err
		}

		for _, targetTimestamp := range manifest {
			logging.WithFields(logrus.Fields{
				"region":           realm.Region.Name,
				"realm":            realm.Slug,
				"manifest":         objAttrs.Name,
				"target-timestamp": targetTimestamp,
			}).Info("Checking")

			rawAuctionsObj := auctionsStoreBaseV2.GetObject(realm, time.Unix(int64(targetTimestamp), 0), rawAuctionsBucket)
			exists, err := auctionsStoreBaseV2.ObjectExists(rawAuctionsObj)
			if err != nil {
				return err
			}

			if !exists {
				return errors.New("timestamp in manifest leads to no obj")
			}
		}
	}

	return nil
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

func BullshitIntake(_ context.Context, m PubSubMessage) error {
	job, err := func() (bus.CollectAuctionsJob, error) {
		var in bus.Message
		if err := json.Unmarshal(m.Data, &in); err != nil {
			return bus.CollectAuctionsJob{}, err
		}

		var out bus.CollectAuctionsJob
		if err := json.Unmarshal([]byte(in.Data), &out); err != nil {
			return bus.CollectAuctionsJob{}, err
		}

		return out, nil
	}()
	if err != nil {
		return err
	}

	logging.WithFields(logrus.Fields{"job": job}).Info("Handling")

	realm := sotah.Realm{
		Realm:  blizzard.Realm{Slug: blizzard.RealmSlug(job.RealmSlug)},
		Region: sotah.Region{Name: blizzard.RegionName(job.RegionName)},
	}

	if realm.Region.Name != "us" || realm.Slug != "earthen-ring" {
		return nil
	}

	return TransferBuckets(realm)
}
