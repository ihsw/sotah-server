package Entity

import (
	"encoding/json"
	"github.com/ihsw/go-download/Cache"
	"strconv"
)

/*
	Item
*/
type Item struct {
	Id      int64
	BlizzId uint64
}

func (self Item) marshal() (string, error) {
	itemJson := ItemJson{
		Id:      self.Id,
		BlizzId: self.BlizzId,
	}

	return itemJson.marshal()
}

/*
	ItemJson
*/
type ItemJson struct {
	Id      int64  `json:"0"`
	BlizzId uint64 `json:"1"`
}

func (self ItemJson) marshal() (string, error) {
	b, err := json.Marshal(self)
	return string(b), err
}

/*
	ItemManager
*/
type ItemManager struct {
	Client Cache.Client
}

func (self ItemManager) Namespace() string { return "item" }

func (self ItemManager) PersistAll(items []Item) ([]Item, error) {
	var (
		err error
		ids []int64
		s   string
	)
	m := self.Client.Main

	// ids
	ids, err = m.IncrAll("item_id", len(items))
	if err != nil {
		return items, err
	}
	for i, id := range ids {
		items[i].Id = id
	}

	// data
	values := make([]Cache.PersistValue, len(items))
	for i, item := range items {
		s, err = item.marshal()
		bucketKey, subKey := Cache.GetBucketKey(item.Id, self.Namespace())
		values[i] = Cache.PersistValue{
			BucketKey: bucketKey,
			SubKey:    subKey,
			Value:     s,
		}
	}
	err = m.PersistAll(values)
	if err != nil {
		return items, err
	}

	// etc
	blizzIds := make([]string, len(items))
	for i, item := range items {
		blizzIds[i] = strconv.FormatInt(item.BlizzId, 10)
	}
	err = m.SAddAll("item:blizz_ids", blizzIds)
	if err != nil {
		return items, err
	}

	return items, nil
}
