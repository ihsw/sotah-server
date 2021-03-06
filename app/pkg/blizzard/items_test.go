package blizzard

import (
	"testing"

	"github.com/sotah-inc/server/app/pkg/utiltest"
	"github.com/stretchr/testify/assert"
)

func validateItem(i Item) bool {
	return i.ID != 0
}

func TestNewItemFromHTTP(t *testing.T) {
	ts, err := utiltest.ServeFile("../TestData/item.json")
	if !assert.Nil(t, err) {
		return
	}

	a, _, err := NewItemFromHTTP(ts.URL)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.True(t, validateItem(a)) {
		return
	}
}
func TestNewItemFromFilepath(t *testing.T) {
	i, err := NewItemFromFilepath("../TestData/item.json")
	if !assert.Nil(t, err) {
		return
	}
	if !assert.True(t, validateItem(i)) {
		return
	}
}
