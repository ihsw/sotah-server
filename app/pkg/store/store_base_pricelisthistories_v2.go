package store

import (
	"fmt"
	"strconv"
	"time"

	"github.com/sotah-inc/server/app/pkg/util"
	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"
	"github.com/sotah-inc/server/app/pkg/blizzard"
	"github.com/sotah-inc/server/app/pkg/sotah"
)

func NewPricelistHistoriesBaseV2(c Client, location string) PricelistHistoriesBaseV2 {
	return PricelistHistoriesBaseV2{base{client: c, location: location}}
}

type PricelistHistoriesBaseV2 struct {
	base
}

func (b PricelistHistoriesBaseV2) getBucketName() string {
	return "pricelist-histories"
}

func (b PricelistHistoriesBaseV2) GetBucket() *storage.BucketHandle {
	return b.base.getBucket(b.getBucketName())
}

func (b PricelistHistoriesBaseV2) GetFirmBucket() (*storage.BucketHandle, error) {
	return b.base.getFirmBucket(b.getBucketName())
}

func (b PricelistHistoriesBaseV2) getObjectName(targetTime time.Time, realm sotah.Realm) string {
	return fmt.Sprintf("%s/%s/%d.txt.gz", realm.Region.Name, realm.Slug, targetTime.Unix())
}

func (b PricelistHistoriesBaseV2) GetObject(targetTime time.Time, realm sotah.Realm, bkt *storage.BucketHandle) *storage.ObjectHandle {
	return b.base.getObject(b.getObjectName(targetTime, realm), bkt)
}

func (b PricelistHistoriesBaseV2) GetFirmObject(targetTime time.Time, realm sotah.Realm, bkt *storage.BucketHandle) (*storage.ObjectHandle, error) {
	return b.base.getFirmObject(b.getObjectName(targetTime, realm), bkt)
}

func (b PricelistHistoriesBaseV2) Handle(aucs blizzard.Auctions, targetTime time.Time, rea sotah.Realm, bkt *storage.BucketHandle) (sotah.UnixTimestamp, error) {
	normalizedTargetDate := sotah.NormalizeTargetDate(targetTime)

	// resolving unix-timestamp of target-time
	targetTimestamp := sotah.UnixTimestamp(targetTime.Unix())

	// gathering an object
	obj := b.GetObject(normalizedTargetDate, rea, bkt)

	// resolving item-price-histories
	ipHistories, err := func() (sotah.ItemPriceHistories, error) {
		exists, err := b.ObjectExists(obj)
		if err != nil {
			return sotah.ItemPriceHistories{}, err
		}

		if !exists {
			return sotah.ItemPriceHistories{}, nil
		}

		reader, err := obj.NewReader(b.client.Context)
		if err != nil {
			return sotah.ItemPriceHistories{}, err
		}
		defer reader.Close()

		return sotah.NewItemPriceHistoriesFromMinimized(reader)
	}()
	if err != nil {
		return 0, err
	}

	// gathering new item-prices from the input
	iPrices := sotah.NewItemPrices(sotah.NewMiniAuctionListFromMiniAuctions(sotah.NewMiniAuctions(aucs)))

	// merging item-prices into the item-price-histories
	for itemId, prices := range iPrices {
		pHistory := func() sotah.PriceHistory {
			result, ok := ipHistories[itemId]
			if !ok {
				return sotah.PriceHistory{}
			}

			return result
		}()
		pHistory[targetTimestamp] = prices

		ipHistories[itemId] = pHistory
	}

	// encoding the item-price-histories for persistence
	gzipEncodedBody, err := ipHistories.EncodeForPersistence()
	if err != nil {
		return 0, err
	}

	// writing it out to the gcloud object
	wc := obj.NewWriter(b.client.Context)
	wc.ContentType = "text/plain"
	wc.ContentEncoding = "gzip"
	if _, err := wc.Write(gzipEncodedBody); err != nil {
		return 0, err
	}
	if err := wc.Close(); err != nil {
		return 0, err
	}

	return sotah.UnixTimestamp(normalizedTargetDate.Unix()), wc.Close()
}

func (b PricelistHistoriesBaseV2) GetAllExpiredTimestamps(
	regionRealms map[blizzard.RegionName]sotah.Realms,
	bkt *storage.BucketHandle,
) (RegionRealmExpiredTimestamps, error) {
	out := make(chan GetExpiredTimestampsJob)
	in := make(chan sotah.Realm)

	// spinning up workers
	worker := func() {
		for realm := range in {
			timestamps, err := b.GetExpiredTimestamps(realm, bkt)
			if err != nil {
				out <- GetExpiredTimestampsJob{
					Err:   err,
					Realm: realm,
				}

				continue
			}

			out <- GetExpiredTimestampsJob{
				Err:        nil,
				Realm:      realm,
				Timestamps: timestamps,
			}
		}
	}
	postWork := func() {
		close(out)
	}
	util.Work(8, worker, postWork)

	// queueing it up
	go func() {
		for _, realms := range regionRealms {
			for _, realm := range realms {
				in <- realm
			}
		}

		close(in)
	}()

	// going over results
	expiredTimestamps := RegionRealmExpiredTimestamps{}
	for job := range out {
		if job.Err != nil {
			return RegionRealmExpiredTimestamps{}, job.Err
		}

		regionName := job.Realm.Region.Name
		if _, ok := expiredTimestamps[regionName]; !ok {
			expiredTimestamps[regionName] = RealmExpiredTimestamps{}
		}

		expiredTimestamps[regionName][job.Realm.Slug] = job.Timestamps
	}

	return expiredTimestamps, nil
}

func (b PricelistHistoriesBaseV2) GetExpiredTimestamps(realm sotah.Realm, bkt *storage.BucketHandle) ([]sotah.UnixTimestamp, error) {
	out := []sotah.UnixTimestamp{}

	limit := sotah.NormalizeTargetDate(time.Now()).AddDate(0, 0, -14)

	prefix := fmt.Sprintf("%s/%s/", realm.Region.Name, realm.Slug)
	it := bkt.Objects(b.client.Context, &storage.Query{Prefix: prefix})
	for {
		objAttrs, err := it.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}

			if err != nil {
				return []sotah.UnixTimestamp{}, err
			}
		}

		targetTimestamp, err := strconv.Atoi(objAttrs.Name[len(prefix):(len(objAttrs.Name) - len(".txt.gz"))])
		if err != nil {
			return []sotah.UnixTimestamp{}, err
		}

		targetTime := time.Unix(int64(targetTimestamp), 0)
		if targetTime.After(limit) {
			continue
		}

		out = append(out, sotah.UnixTimestamp(targetTimestamp))
	}

	return out, nil
}
