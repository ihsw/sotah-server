package subjects

// Subject - typehint for these enums
type Subject string

/*
Status - subject name for returning current status
*/
const (
	Status                          Subject = "status"
	Auctions                        Subject = "auctions"
	GenericTestErrors               Subject = "genericTestErrors"
	Owners                          Subject = "owners"
	OwnersQueryByItems              Subject = "ownersQueryByItems"
	OwnersQuery                     Subject = "ownersQuery"
	ItemsQuery                      Subject = "itemsQuery"
	PriceList                       Subject = "priceList"
	PriceListHistory                Subject = "priceListHistory"
	PriceListHistoryV2              Subject = "priceListHistoryV2"
	Items                           Subject = "items"
	Boot                            Subject = "boot"
	SessionSecret                   Subject = "sessionSecret"
	RuntimeInfo                     Subject = "runtimeInfo"
	LiveAuctionsIntake              Subject = "liveAuctionsIntake"
	LiveAuctionsIntakeV2            Subject = "liveAuctionsIntakeV2"
	LiveAuctionsCompute             Subject = "liveAuctionsCompute"
	LiveAuctionsComputeIntake       Subject = "liveAuctionsComputeIntake"
	PricelistHistoriesIntake        Subject = "pricelistHistoriesIntake"
	AppMetrics                      Subject = "appMetrics"
	PricelistHistoriesIntakeV2      Subject = "pricelistHistoriesIntakeV2"
	PricelistHistoriesCompute       Subject = "pricelistHistoriesCompute"
	PricelistHistoriesComputeIntake Subject = "pricelistHistoriesComputeIntake"
)

// gcloud fn-related
const (
	DownloadAllAuctions Subject = "downloadAllAuctions"
	DownloadAuctions    Subject = "downloadAuctions"

	SyncAllItems       Subject = "syncAllItems"
	SyncItem           Subject = "syncItem"
	ReceiveSyncedItems Subject = "receiveSyncedItems"

	ComputeAllLiveAuctions      Subject = "computeAllLiveAuctions"
	ComputeLiveAuctions         Subject = "computeLiveAuctions"
	ReceiveComputedLiveAuctions Subject = "receiveComputedLiveAuctions"

	ComputeAllPricelistHistories      Subject = "computeAllPricelistHistories"
	ComputePricelistHistories         Subject = "computePricelistHistories"
	ReceiveComptuedPricelistHistories Subject = "receiveComputedPricelistHistories"

	CleanupAllExpiredManifests Subject = "cleanupAllExpiredManifests"
	CleanupExpiredManifest     Subject = "cleanupExpiredManifest"
)
