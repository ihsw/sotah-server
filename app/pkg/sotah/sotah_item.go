package sotah

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/sotah-inc/server/app/pkg/blizzard"
	"github.com/sotah-inc/server/app/pkg/util"
)

func NormalizeName(in string) (string, error) {
	reg, err := regexp.Compile("[^a-z0-9 ]+")
	if err != nil {
		return "", err
	}

	return reg.ReplaceAllString(strings.ToLower(in), ""), nil
}

// item
func NewItem(body []byte) (Item, error) {
	i := &Item{}
	if err := json.Unmarshal(body, i); err != nil {
		return Item{}, err
	}

	return *i, nil
}

type Item struct {
	blizzard.Item

	IconURL        string `json:"icon_url"`
	IconObjectName string `json:"icon_object_name"`
	LastModified   int    `json:"last_modified"`
}

// item-icon-item-ids map
type ItemIconItemIdsMap map[string][]blizzard.ItemID

func (iconsMap ItemIconItemIdsMap) GetItemIcons() []string {
	iconNames := make([]string, len(iconsMap))
	i := 0
	for iconName := range iconsMap {
		iconNames[i] = iconName

		i++
	}

	return iconNames
}

// item-ids map
func NewItemIdsMap(IDs []blizzard.ItemID) ItemIdsMap {
	out := ItemIdsMap{}

	for _, ID := range IDs {
		out[ID] = struct{}{}
	}

	return out
}

type ItemIdsMap map[blizzard.ItemID]struct{}

// items-map
func NewItemsMapFromGzipped(body []byte) (ItemsMap, error) {
	gzipDecodedData, err := util.GzipDecode(body)
	if err != nil {
		return ItemsMap{}, err
	}

	return newItemsMap(gzipDecodedData)
}

func newItemsMap(body []byte) (ItemsMap, error) {
	iMap := &ItemsMap{}
	if err := json.Unmarshal(body, iMap); err != nil {
		return nil, err
	}

	return *iMap, nil
}

type ItemsMap map[blizzard.ItemID]Item

func (iMap ItemsMap) getItemIds() []blizzard.ItemID {
	out := []blizzard.ItemID{}
	for ID := range iMap {
		out = append(out, ID)
	}

	return out
}

func (iMap ItemsMap) GetItemIconsMap(excludeWithURL bool) ItemIconItemIdsMap {
	out := ItemIconItemIdsMap{}
	for itemId, iValue := range iMap {
		if excludeWithURL && iValue.IconURL != "" {
			continue
		}

		if iValue.Icon == "" {
			continue
		}

		if _, ok := out[iValue.Icon]; !ok {
			out[iValue.Icon] = []blizzard.ItemID{itemId}

			continue
		}

		out[iValue.Icon] = append(out[iValue.Icon], itemId)
	}

	return out
}

func (iMap ItemsMap) EncodeForDatabase() ([]byte, error) {
	jsonEncodedData, err := json.Marshal(iMap)
	if err != nil {
		return []byte{}, err
	}

	gzipEncodedData, err := util.GzipEncode(jsonEncodedData)
	if err != nil {
		return []byte{}, err
	}

	return gzipEncodedData, nil
}

func NewItemIdNameMap(data string) (ItemIdNameMap, error) {
	base64Decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return ItemIdNameMap{}, err
	}

	gzipDecoded, err := util.GzipDecode(base64Decoded)
	if err != nil {
		return ItemIdNameMap{}, err
	}

	var out ItemIdNameMap
	if err := json.Unmarshal(gzipDecoded, &out); err != nil {
		return ItemIdNameMap{}, err
	}

	return out, nil
}

type ItemIdNameMap map[blizzard.ItemID]string

func (idNameMap ItemIdNameMap) EncodeForDelivery() (string, error) {
	jsonEncodedData, err := json.Marshal(idNameMap)
	if err != nil {
		return "", err
	}

	gzipEncodedData, err := util.GzipEncode(jsonEncodedData)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(gzipEncodedData), nil
}
