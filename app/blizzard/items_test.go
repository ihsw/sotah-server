package blizzard

import (
	"testing"

	"github.com/ihsw/sotah-server/app/utiltest"
	"github.com/stretchr/testify/assert"
)

func validateItem(i item) bool {
	return i.ID != 0
}

func TestNewItemFromHTTP(t *testing.T) {
	ts, err := utiltest.ServeFile("../TestData/item.json")
	if !assert.Nil(t, err) {
		return
	}

	a, err := newItemFromHTTP(ts.URL)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.True(t, validateItem(a)) {
		return
	}
}
func TestNewItemFromFilepath(t *testing.T) {
	i, err := newItemFromFilepath("../TestData/item.json")
	if !assert.Nil(t, err) {
		return
	}
	if !assert.True(t, validateItem(i)) {
		return
	}
}