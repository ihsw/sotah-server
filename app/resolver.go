package main

import (
	"net/url"

	"github.com/ihsw/sotah-server/app/blizzard"
	"github.com/ihsw/sotah-server/app/util"
)

func newResolver(c config) resolver {
	return resolver{
		config: &c,

		getStatusURL:      blizzard.DefaultGetStatusURL,
		getAuctionInfoURL: defaultGetAuctionInfoURL,
		getAuctionsURL:    defaultGetAuctionsURL,
		getItemURL:        blizzard.DefaultGetItemURL,
		getItemIconURL:    defaultGetItemIconURL,
		getItemClassesURL: defaultGetItemClassesURL,
	}
}

func (r resolver) appendAPIKey(destination string) (string, error) {
	u, err := url.Parse(destination)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("apikey", r.config.APIKey)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (r resolver) get(destination string) ([]byte, error) {
	if r.config != nil && len(r.config.APIKey) > 0 {
		var err error
		destination, err = r.appendAPIKey(destination)
		if err != nil {
			return []byte{}, err
		}
	}

	return util.Download(destination)
}

type resolver struct {
	config *config

	getStatusURL      blizzard.GetStatusURLFunc
	getAuctionInfoURL getAuctionInfoURLFunc
	getAuctionsURL    getAuctionsURLFunc
	getItemURL        blizzard.GetItemURLFunc
	getItemIconURL    getItemIconURLFunc
	getItemClassesURL getItemClassesURLFunc
}
