package main

import (
	"time"

	"github.com/ihsw/sotah-server/app/blizzard"
	log "github.com/sirupsen/logrus"
)

func (sta state) startCollector(stopChan workerStopChan, res resolver) workerStopChan {
	// collecting regions once
	sta.collectRegions(res)

	onStop := make(workerStopChan)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)

		log.Info("Starting collector")
	outer:
		for {
			select {
			case <-ticker.C:
				sta.collectRegions(res)
			case <-stopChan:
				ticker.Stop()

				break outer
			}
		}

		onStop <- struct{}{}
	}()

	return onStop
}

func (sta state) collectRegions(res resolver) {
	log.Info("Collecting regions")

	// gathering the total number of auctions pre-collection
	totalPreviousAuctions := 0
	for _, reg := range sta.regions {
		for _, rea := range sta.statuses[reg.Name].Realms {
			for _, auc := range sta.auctions[reg.Name][rea.Slug] {
				totalPreviousAuctions += len(auc.AucList)
			}
		}
	}

	// going over the list of regions
	startTime := time.Now()
	totalChurnAmount := 0
	for _, reg := range sta.regions {
		// gathering whitelist for this region
		wList := res.config.getRegionWhitelist(reg)
		if wList != nil && len(*wList) == 0 {
			continue
		}

		// misc
		regionItemIDsMap := map[blizzard.ItemID]struct{}{}

		// downloading auctions in a region
		log.WithFields(log.Fields{
			"region":    reg.Name,
			"realms":    len(sta.statuses[reg.Name].Realms),
			"whitelist": wList,
		}).Info("Downloading region")
		auctionsOut := sta.statuses[reg.Name].Realms.getAuctionsOrAll(sta.resolver, wList)
		for job := range auctionsOut {
			result := sta.auctionsIntake(job)
			totalChurnAmount += result.removedAuctionsCount
			for _, ID := range result.itemIds {
				_, ok := sta.items[ID]
				if ok {
					continue
				}

				regionItemIDsMap[ID] = struct{}{}
			}
		}
		log.WithField("region", reg.Name).Info("Downloaded region")

		// optionally gathering the list of item IDs for this region
		if len(regionItemIDsMap) > 0 {
			regionItemIDs := make([]blizzard.ItemID, len(regionItemIDsMap))
			i := 0
			for ID := range regionItemIDsMap {
				regionItemIDs[i] = ID
				i++
			}

			// downloading items found in this region
			log.WithField("items", len(regionItemIDs)).Info("Fetching items")
			itemsOut := getItems(regionItemIDs, res)
			for job := range itemsOut {
				if job.err != nil {
					log.WithFields(log.Fields{
						"region": reg.Name,
						"item":   job.ID,
						"error":  job.err.Error(),
					}).Info("Failed to fetch item")

					continue
				}

				sta.items[job.ID] = job.item
			}
			log.WithField("items", len(regionItemIDs)).Info("Fetched items")
		}
	}

	// re-syncing all item icons
	iconNames := sta.items.getItemIcons()
	log.WithField("items", len(iconNames)).Info("Syncing item icons")
	itemIconsOut := syncItemIcons(iconNames, res)
	for job := range itemIconsOut {
		if job.err != nil {
			log.WithFields(log.Fields{
				"item":  job.icon,
				"error": job.err.Error(),
			}).Info("Failed to sync item icon")

			continue
		}
	}
	log.WithField("items", len(iconNames)).Info("Synced item icons")

	// gathering owner, item, and storage metrics
	totalOwners := 0
	currentItemIds := map[blizzard.ItemID]struct{}{}
	totalAuctions := 0
	totalStorageSize := int64(0)
	for _, reg := range sta.regions {
		for _, rea := range sta.statuses[reg.Name].Realms {
			realmAuctions := sta.auctions[reg.Name][rea.Slug]
			realmOwnerNames := map[ownerName]struct{}{}
			for _, auc := range realmAuctions {
				realmOwnerNames[auc.Owner] = struct{}{}
				currentItemIds[auc.Item.ID] = struct{}{}
				totalAuctions += len(auc.AucList)
			}
			totalOwners += len(realmOwnerNames)

			if res.config.UseGCloudStorage {
				totalRealmStorageSize, err := res.store.getTotalRealmAuctionsSize(rea)
				if err != nil {
					log.WithFields(log.Fields{
						"region": rea.region.Name,
						"realm":  rea.Slug,
						"error":  err.Error(),
					}).Info("Failed to fetch total realm auction size")
				}

				totalStorageSize += totalRealmStorageSize
			}
		}
	}

	// calculating churn ratio
	churnRatio := float32(0)
	if totalPreviousAuctions > 0 {
		churnRatio = float32(totalChurnAmount) / float32(totalPreviousAuctions)
	}

	sta.messenger.publishMetric(telegrafMetrics{
		"item_count":          int64(len(sta.items)),
		"current_owner_count": int64(totalOwners),
		"current_item_count":  int64(len(currentItemIds)),
		"collector_duration":  int64(time.Now().Unix() - startTime.Unix()),
		"total_churn_amount":  int64(totalChurnAmount),
		"churn_ratio":         int64(churnRatio * 1000),
		"total_auctions":      int64(totalAuctions),
		"total_storage_size":  totalStorageSize,
	})
}
