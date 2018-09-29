package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/ihsw/sotah-server/app/blizzard"
	"github.com/ihsw/sotah-server/app/logging"
	"github.com/ihsw/sotah-server/app/subjects"
	"github.com/ihsw/sotah-server/app/util"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
)

func apiCacheDirs(c config, regions regionList) ([]string, error) {
	databaseDir, err := c.databaseDir()
	if err != nil {
		return nil, err
	}

	cacheDirs := []string{databaseDir}
	if !c.UseGCloudStorage {
		cacheDirs = append(cacheDirs, fmt.Sprintf("%s/auctions", c.CacheDir))

		for _, reg := range regions {
			cacheDirs = append(cacheDirs, fmt.Sprintf("%s/auctions/%s", c.CacheDir, reg.Name))
		}
	}

	return cacheDirs, nil
}

func api(c config, m messenger, s store) error {
	logging.Info("Starting api")

	// establishing a state
	res := newResolver(c, m, s)
	sta := newState(m, res)

	// creating a uuid4 api-session secret
	sta.sessionSecret = uuid.NewV4()

	// ensuring cache-dirs exist
	cacheDirs, err := apiCacheDirs(c, sta.regions)
	if err != nil {
		return err
	}

	if err := util.EnsureDirsExist(cacheDirs); err != nil {
		return err
	}

	// loading up items database
	idBase, err := newItemsDatabase(c)
	if err != nil {
		return err
	}
	sta.itemsDatabase = idBase

	// filling state with region statuses
	for _, reg := range sta.regions {
		regionStatus, err := reg.getStatus(res)
		if err != nil {
			logging.WithFields(logrus.Fields{
				"error":  err.Error(),
				"region": reg.Name,
			}).Error("Failed to fetch status")

			return err
		}

		regionStatus.Realms = regionStatus.Realms
		sta.statuses[reg.Name] = regionStatus
	}

	// gathering item-classes
	primaryRegion, err := c.Regions.getPrimaryRegion()
	if err != nil {
		return err
	}

	uri, err := res.appendAPIKey(res.getItemClassesURL(primaryRegion.Hostname))
	if err != nil {
		return err
	}

	iClasses, resp, err := blizzard.NewItemClassesFromHTTP(uri)
	if err != nil {
		return err
	}

	sta.itemClasses = iClasses
	if err := sta.messenger.publishPlanMetaMetric(resp); err != nil {
		return err
	}

	// gathering profession icons into storage
	if c.UseGCloudStorage {
		iconNames := make([]string, len(c.Professions))
		for i, prof := range c.Professions {
			iconNames[i] = prof.Icon
		}

		syncedIcons, err := s.syncItemIcons(iconNames, res)
		if err != nil {
			return err
		}
		for job := range syncedIcons {
			if job.err != nil {
				return job.err
			}

			for i, prof := range c.Professions {
				if prof.Icon != job.iconName {
					continue
				}

				c.Professions[i].IconURL = job.iconURL
			}
		}
	} else {
		for i, prof := range c.Professions {
			c.Professions[i].IconURL = defaultGetItemIconURL(prof.Icon)
		}
	}

	// opening all listeners
	sta.listeners = newListeners(subjectListeners{
		subjects.GenericTestErrors: sta.listenForGenericTestErrors,
		subjects.Status:            sta.listenForStatus,
		subjects.Regions:           sta.listenForRegions,
		subjects.ItemsQuery:        sta.listenForItemsQuery,
		subjects.ItemClasses:       sta.listenForItemClasses,
		subjects.Items:             sta.listenForItems,
		subjects.Boot:              sta.listenForBoot,
		subjects.SessionSecret:     sta.listenForSessionSecret,
	})
	if err := sta.listeners.listen(); err != nil {
		return err
	}

	// starting up a collector
	collectorStop := make(workerStopChan)
	onCollectorStop := sta.startCollector(collectorStop, res)

	// catching SIGINT
	logging.Info("Waiting for SIGINT")
	sigIn := make(chan os.Signal, 1)
	signal.Notify(sigIn, os.Interrupt)
	<-sigIn

	logging.Info("Caught SIGINT, exiting")

	// stopping listeners
	sta.listeners.stop()

	logging.Info("Stopping collector")
	collectorStop <- struct{}{}

	logging.Info("Waiting for collector to stop")
	<-onCollectorStop

	logging.Info("Exiting")
	return nil
}
