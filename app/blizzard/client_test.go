package blizzard

import (
	"testing"

	"github.com/sotah-inc/server/app/utiltest"
	"github.com/stretchr/testify/assert"
)

func TestClientRefresh(t *testing.T) {
	client := NewClient("", "")

	ts, err := utiltest.ServeFile("../TestData/access-token.json")
	if !assert.Nil(t, err) {
		return
	}

	err = client.RefreshFromHTTP(ts.URL)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Empty(t, client.accessToken) {
		return
	}
}