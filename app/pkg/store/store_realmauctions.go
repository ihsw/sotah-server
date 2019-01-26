package store

import (
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"github.com/sotah-inc/server/app/pkg/blizzard"
	"github.com/sotah-inc/server/app/pkg/logging"
	"github.com/sotah-inc/server/app/pkg/sotah"
)

func (sto Store) getRealmAuctionsBucketName(rea sotah.Realm) string {
	return fmt.Sprintf("raw-auctions_%s_%s", rea.Region.Name, rea.Slug)
}

func (sto Store) GetRealmAuctionsBucket(rea sotah.Realm) *storage.BucketHandle {
	return sto.client.Bucket(sto.getRealmAuctionsBucketName(rea))
}

func (sto Store) createRealmAuctionsBucket(rea sotah.Realm) (*storage.BucketHandle, error) {
	bkt := sto.GetRealmAuctionsBucket(rea)
	err := bkt.Create(sto.Context, sto.projectID, &storage.BucketAttrs{
		StorageClass: "REGIONAL",
		Location:     "us-east1",
	})
	if err != nil {
		return nil, err
	}

	return bkt, nil
}

func (sto Store) RealmAuctionsBucketExists(rea sotah.Realm) (bool, error) {
	_, err := sto.GetRealmAuctionsBucket(rea).Attrs(sto.Context)
	if err != nil {
		if err != storage.ErrBucketNotExist {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (sto Store) resolveRealmAuctionsBucket(rea sotah.Realm) (*storage.BucketHandle, error) {
	exists, err := sto.RealmAuctionsBucketExists(rea)
	if err != nil {
		return nil, err
	}

	if !exists {
		return sto.createRealmAuctionsBucket(rea)
	}

	return sto.GetRealmAuctionsBucket(rea), nil
}

func (sto Store) GetRealmAuctionsObjectName(lastModified time.Time) string {
	return fmt.Sprintf("%d.json.gz", lastModified.Unix())
}

func (sto Store) getRealmAuctionsObject(bkt *storage.BucketHandle, lastModified time.Time) *storage.ObjectHandle {
	return bkt.Object(sto.GetRealmAuctionsObjectName(lastModified))
}

func (sto Store) realmAuctionsObjectExists(bkt *storage.BucketHandle, lastModified time.Time) (bool, error) {
	_, err := sto.getRealmAuctionsObject(bkt, lastModified).Attrs(sto.Context)
	if err != nil {
		if err != storage.ErrObjectNotExist {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (sto Store) getRealmAuctionsObjectAtTime(bkt *storage.BucketHandle, targetTime time.Time) (*storage.ObjectHandle, error) {
	logging.WithField("targetTime", targetTime.Unix()).Debug("Fetching realm-auctions object at time")

	exists, err := sto.realmAuctionsObjectExists(bkt, targetTime)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errors.New("realm auctions object does not exist")
	}

	return sto.getRealmAuctionsObject(bkt, targetTime), nil
}

func (sto Store) WriteRealmAuctions(rea sotah.Realm, lastModified time.Time, gzipEncodedBody []byte) error {
	bkt, err := sto.resolveRealmAuctionsBucket(rea)
	if err != nil {
		return err
	}

	logging.WithFields(logrus.Fields{
		"region": rea.Region.Name,
		"realm":  rea.Slug,
		"length": len(gzipEncodedBody),
	}).Debug("Writing auctions to gcloud storage")

	wc := bkt.Object(sto.GetRealmAuctionsObjectName(lastModified)).NewWriter(sto.Context)
	wc.ContentType = "application/json"
	wc.ContentEncoding = "gzip"

	if _, err := wc.Write(gzipEncodedBody); err != nil {
		return err
	}

	return wc.Close()
}

func (sto Store) getAuctions(rea sotah.Realm, targetTime time.Time) (blizzard.Auctions, error) {
	hasBucket, err := sto.RealmAuctionsBucketExists(rea)
	if err != nil {
		return blizzard.Auctions{}, err
	}

	if !hasBucket {
		logging.WithFields(logrus.Fields{
			"region": rea.Region.Name,
			"realm":  rea.Slug,
		}).Error("Realm has no bucket")

		return blizzard.Auctions{}, errors.New("realm has no bucket")
	}

	bkt := sto.GetRealmAuctionsBucket(rea)
	if err != nil {
		return blizzard.Auctions{}, err
	}

	obj, err := sto.getRealmAuctionsObjectAtTime(bkt, targetTime)
	if err != nil {
		return blizzard.Auctions{}, err
	}

	if obj == nil {
		logging.WithFields(logrus.Fields{
			"region": rea.Region.Name,
			"realm":  rea.Slug,
		}).Error("Found no auctions in Store")

		return blizzard.Auctions{}, errors.New("found no auctions in store at specified time")
	}

	logging.WithFields(logrus.Fields{
		"region": rea.Region.Name,
		"realm":  rea.Slug,
	}).Debug("Loading auctions from Store")

	return sto.NewAuctions(obj)
}

func (sto Store) NewAuctions(obj *storage.ObjectHandle) (blizzard.Auctions, error) {
	reader, err := obj.NewReader(sto.Context)
	if err != nil {
		return blizzard.Auctions{}, err
	}
	defer reader.Close()

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return blizzard.Auctions{}, err
	}

	return blizzard.NewAuctions(body)
}
