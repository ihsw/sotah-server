package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ihsw/sotah-server/app/blizzard"
	"github.com/ihsw/sotah-server/app/codes"
	"github.com/ihsw/sotah-server/app/subjects"
	"github.com/ihsw/sotah-server/app/util"
	log "github.com/sirupsen/logrus"
)

type getAuctionsWhitelist map[blizzard.RealmSlug]interface{}

type getAuctionsJob struct {
	err      error
	realm    realm
	auctions blizzard.Auctions
}

func newRealms(reg region, blizzRealms []blizzard.Realm) realms {
	reas := make([]realm, len(blizzRealms))
	for i, rea := range blizzRealms {
		reas[i] = realm{rea, reg}
	}

	return reas
}

type realms []realm

func (reas realms) getAuctionsOrAll(res resolver, wList getAuctionsWhitelist) chan getAuctionsJob {
	if wList == nil {
		return reas.getAllAuctions(res)
	}

	return reas.getAuctions(res, wList)
}

func (reas realms) getAllAuctions(res resolver) chan getAuctionsJob {
	wList := map[blizzard.RealmSlug]interface{}{}
	for _, rea := range reas {
		wList[rea.Slug] = true
	}
	return reas.getAuctions(res, wList)
}

func (reas realms) getAuctions(res resolver, wList getAuctionsWhitelist) chan getAuctionsJob {
	// establishing channels
	out := make(chan getAuctionsJob)
	in := make(chan realm)

	// spinning up the workers for fetching auctions
	worker := func() {
		for rea := range in {
			aucs, err := rea.getAuctions(res)
			log.WithField("realm", rea.Slug).Debug("Received auctions")
			out <- getAuctionsJob{err: err, realm: rea, auctions: aucs}
		}
	}
	postWork := func() {
		close(out)
	}
	util.Work(4, worker, postWork)

	// queueing up the realms
	go func() {
		for _, rea := range reas {
			if _, ok := wList[rea.Slug]; !ok {
				continue
			}

			log.WithField("realm", rea.Slug).Debug("Queueing up auction for downloading")
			in <- rea
		}

		close(in)
	}()

	return out
}

type realm struct {
	blizzard.Realm
	region region
}

func (rea realm) LogEntry() *log.Entry {
	return log.WithFields(log.Fields{"region": rea.region.Name, "realm": rea.Slug})
}

func (rea realm) auctionsFilepath(c *config) (string, error) {
	return filepath.Abs(
		fmt.Sprintf("%s/auctions/%s/%s.json.gz", c.CacheDir, rea.region.Name, rea.Slug),
	)
}

func (rea realm) getAuctions(res resolver) (blizzard.Auctions, error) {
	uri, err := res.appendAPIKey(res.getAuctionInfoURL(rea.region.Hostname, rea.Slug))
	if err != nil {
		return blizzard.Auctions{}, err
	}

	// resolving auction-info from the api
	aInfo, err := blizzard.NewAuctionInfoFromHTTP(uri)
	if err != nil {
		return blizzard.Auctions{}, err
	}

	// validating the list of files
	if len(aInfo.Files) == 0 {
		return blizzard.Auctions{}, errors.New("Cannot fetch auctions with blank files")
	}
	aFile := aInfo.Files[0]

	// validating config
	if res.config == nil {
		return blizzard.Auctions{}, errors.New("Config cannot be nil")
	}

	// optionally falling back to fetching from the api where use-cache-dir is off
	if res.config.UseCacheDir == false {
		uri, err := res.appendAPIKey(res.getAuctionsURL(aFile.URL))
		if err != nil {
			return blizzard.Auctions{}, err
		}

		return blizzard.NewAuctionsFromHTTP(uri)
	}

	// validating the cache dir pathname
	if res.config.CacheDir == "" {
		return blizzard.Auctions{}, errors.New("Cache dir cannot be blank")
	}

	// validating the realm region
	if rea.region.Name == "" {
		return blizzard.Auctions{}, errors.New("Region name cannot be blank")
	}

	// resolving the auctions filepath
	auctionsFilepath, err := rea.auctionsFilepath(res.config)
	if err != nil {
		return blizzard.Auctions{}, err
	}

	// stating the auction file
	fInfo, err := os.Stat(auctionsFilepath)
	if err != nil && !os.IsNotExist(err) {
		return blizzard.Auctions{}, err
	}

	if fInfo.ModTime().Before(aFile.LastModifiedAsTime()) {
		body, err := res.get(res.getAuctionsURL(aFile.URL))
		if err != nil {
			return blizzard.Auctions{}, err
		}

		encodedBody, err := util.GzipEncode(body)
		if err != nil {
			return blizzard.Auctions{}, err
		}

		log.WithFields(log.Fields{
			"region": rea.region.Name,
			"realm":  rea.Slug,
		}).Debug("Writing auction data to cache dir")
		if err := util.WriteFile(auctionsFilepath, encodedBody); err != nil {
			return blizzard.Auctions{}, err
		}

		return blizzard.NewAuctions(body)
	}

	rea.LogEntry().Debug("Loading auction data from cache dir")
	return blizzard.NewAuctionsFromGzFilepath(auctionsFilepath)
}

func newStatusFromMessenger(reg region, mess messenger) (status, error) {
	lm := statusRequest{RegionName: reg.Name}
	encodedMessage, err := json.Marshal(lm)
	if err != nil {
		return status{}, err
	}

	msg, err := mess.request(subjects.Status, encodedMessage)
	if err != nil {
		return status{}, err
	}

	if msg.Code != codes.Ok {
		return status{}, errors.New(msg.Err)
	}

	stat, err := blizzard.NewStatus([]byte(msg.Data))
	if err != nil {
		return status{}, err
	}

	return newStatus(reg, stat), nil
}

func newStatusFromFilepath(reg region, relativeFilepath string) (status, error) {
	stat, err := blizzard.NewStatusFromFilepath(relativeFilepath)
	if err != nil {
		return status{}, err
	}

	return newStatus(reg, stat), nil
}

func newStatus(reg region, stat blizzard.Status) status {
	return status{stat, reg, newRealms(reg, stat.Realms)}
}

type status struct {
	blizzard.Status
	region region
	Realms realms
}
