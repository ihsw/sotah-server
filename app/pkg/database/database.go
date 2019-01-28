package database

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sotah-inc/server/app/pkg/blizzard"
	"github.com/sotah-inc/server/app/pkg/sotah"

	"github.com/sirupsen/logrus"
	"github.com/sotah-inc/server/app/pkg/logging"
)

func normalizeTargetDate(targetDate time.Time) time.Time {
	nearestWeekStartOffset := targetDate.Second() + targetDate.Minute()*60 + targetDate.Hour()*60*60
	return time.Unix(targetDate.Unix()-int64(nearestWeekStartOffset), 0)
}

func DatabaseRetentionLimit() time.Time {
	return time.Now().Add(-1 * time.Hour * 24 * 30)
}

type databasePathPair struct {
	FullPath   string
	TargetTime time.Time
}

func DatabasePaths(databaseDir string) ([]databasePathPair, error) {
	out := []databasePathPair{}

	databaseFilepaths, err := ioutil.ReadDir(databaseDir)
	if err != nil {
		logging.WithFields(logrus.Fields{
			"error": err.Error(),
			"dir":   databaseDir,
		}).Error("Failed to read database dir")

		return []databasePathPair{}, err
	}

	for _, fPath := range databaseFilepaths {
		if !strings.HasPrefix(fPath.Name(), "next-") {
			continue
		}

		parts := strings.Split(fPath.Name(), ".")
		parts = strings.Split(parts[0], "-")
		targetTimeUnix, err := strconv.Atoi(parts[1])
		if err != nil {
			logging.WithFields(logrus.Fields{
				"error":    err.Error(),
				"dir":      databaseDir,
				"pathname": fPath.Name(),
			}).Error("Failed to parse database filepath")

			return []databasePathPair{}, err
		}

		targetTime := time.Unix(int64(targetTimeUnix), 0)

		fullPath, err := filepath.Abs(fmt.Sprintf("%s/%s", databaseDir, fPath.Name()))
		if err != nil {
			logging.WithFields(logrus.Fields{
				"error":    err.Error(),
				"dir":      databaseDir,
				"pathname": fPath.Name(),
			}).Error("Failed to resolve full path of database file")

			return []databasePathPair{}, err
		}

		out = append(out, databasePathPair{fullPath, targetTime})
	}

	return out, nil
}

type LoadInJob struct {
	Realm      sotah.Realm
	TargetTime time.Time
	Auctions   blizzard.Auctions
}