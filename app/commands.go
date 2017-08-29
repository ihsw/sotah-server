package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/ihsw/sotah-server/app/subjects"
	log "github.com/sirupsen/logrus"
)

func apiTest(c *config, m messenger, dataDir string) error {
	log.Info("Starting api-test")

	dataDirPath, err := filepath.Abs(dataDir)
	if err != nil {
		return err
	}

	// establishing a state and filling it with statuses
	sta := state{
		messenger: m,
		regions:   c.Regions,
		statuses:  map[regionName]*status{},
	}
	for _, reg := range c.Regions {
		stat, err := newStatusFromFilepath(reg, fmt.Sprintf("%s/realm-status.json", dataDirPath))
		if err != nil {
			return err
		}

		sta.statuses[reg.Name] = stat
	}

	// listening for status requests
	stopChans := map[string]chan interface{}{
		subjects.Status:            make(chan interface{}),
		subjects.Regions:           make(chan interface{}),
		subjects.GenericTestErrors: make(chan interface{}),
	}
	if err := sta.listenForStatus(stopChans[subjects.Status]); err != nil {
		return err
	}
	if err := sta.listenForRegions(stopChans[subjects.Regions]); err != nil {
		return err
	}
	if err := sta.listenForGenericTestErrors(stopChans[subjects.GenericTestErrors]); err != nil {
		return err
	}

	// catching SIGINT
	sigIn := make(chan os.Signal, 1)
	signal.Notify(sigIn, os.Interrupt)
	<-sigIn

	log.Info("Caught SIGINT, exiting")

	// stopping listeners
	for _, stop := range stopChans {
		stop <- struct{}{}
	}

	return nil
}