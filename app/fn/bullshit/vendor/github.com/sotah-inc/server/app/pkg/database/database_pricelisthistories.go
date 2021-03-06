package database

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/sotah-inc/server/app/pkg/blizzard"
	"github.com/sotah-inc/server/app/pkg/sotah"
)

// keying
func pricelistHistoryKeyName() []byte {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, 1)

	return key
}

// bucketing
func pricelistHistoryBucketName(ID blizzard.ItemID) []byte {
	return []byte(fmt.Sprintf("item-prices/%d", ID))
}

// db
func pricelistHistoryDatabaseFilePath(
	dirPath string,
	regionName blizzard.RegionName,
	realmSlug blizzard.RealmSlug,
	targetTimestamp sotah.UnixTimestamp,
) string {
	return fmt.Sprintf(
		"%s/pricelist-histories/%s/%s/%d.db",
		dirPath,
		regionName,
		realmSlug,
		targetTimestamp,
	)
}

func NewPricelistHistoryDatabases(dirPath string, statuses sotah.Statuses) (PricelistHistoryDatabases, error) {
	if len(dirPath) == 0 {
		return PricelistHistoryDatabases{}, errors.New("dir-path cannot be blank")
	}

	phdBases := PricelistHistoryDatabases{
		databaseDir: dirPath,
		Databases:   regionRealmDatabaseShards{},
	}

	for regionName, regionStatuses := range statuses {
		phdBases.Databases[regionName] = realmDatabaseShards{}

		for _, rea := range regionStatuses.Realms {
			phdBases.Databases[regionName][rea.Slug] = PricelistHistoryDatabaseShards{}

			dbPathPairs, err := Paths(fmt.Sprintf("%s/pricelist-histories/%s/%s", dirPath, regionName, rea.Slug))
			if err != nil {
				return PricelistHistoryDatabases{}, err
			}

			for _, dbPathPair := range dbPathPairs {
				phdBase, err := newPricelistHistoryDatabase(dbPathPair.FullPath, dbPathPair.TargetTime)
				if err != nil {
					return PricelistHistoryDatabases{}, err
				}

				phdBases.Databases[regionName][rea.Slug][sotah.UnixTimestamp(dbPathPair.TargetTime.Unix())] = phdBase
			}
		}
	}

	return phdBases, nil
}
