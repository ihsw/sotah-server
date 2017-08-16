package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
)

func main() {
	// parsing the command flags
	configFilepath := flag.String("config", "", "Relative path to config json")
	natsHost := flag.String("nats_host", "", "Hostname of nats server")
	natsPort := flag.Int("nats_port", 0, "Port number of nats server")
	flag.Parse()

	// loading the config file
	config, err := newConfigFromFilepath(*configFilepath)
	if err != nil {
		log.Fatalf("Could not fetch config: %s\n", err.Error())

		return
	}
	res := newResolver(config.APIKey)

	// connecting the messenger
	messenger, err := newMessenger(*natsHost, *natsPort)
	if err != nil {
		log.Fatalf("Could not connect messenger: %s\n", err.Error())

		return
	}

	// establishing a state and filling it with statuses
	sta := state{messenger: messenger, statuses: map[regionName]*status{}}
	for _, reg := range config.Regions {
		stat, err := newStatusFromHTTP(reg, res)
		if err != nil {
			log.Fatalf("Could not fetch statuses from http: %s\n", err.Error())

			return
		}

		sta.statuses[reg.Name] = stat
	}

	// listening for status requests
	stop := make(chan interface{})
	if err := sta.listenForStatus(stop); err != nil {
		log.Fatalf("Could not listen for status requests: %s\n", err.Error())

		return
	}

	// catching SIGINT
	sigIn := make(chan os.Signal, 1)
	signal.Notify(sigIn, os.Interrupt)
	<-sigIn
	fmt.Printf("Caught SIGINT!")

	// stopping status listener
	stop <- struct{}{}

	// exiting
	os.Exit(0)
}
