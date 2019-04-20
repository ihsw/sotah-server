package state

import (
	"cloud.google.com/go/storage"
	"github.com/sotah-inc/server/app/pkg/logging"
	"github.com/sotah-inc/server/app/pkg/store"
	"github.com/sotah-inc/server/app/pkg/util"
	"github.com/twinj/uuid"
	"google.golang.org/api/iterator"
)

type TransferStateConfig struct {
	InProjectId  string
	InBucketName string

	OutProjectId  string
	OutBucketName string
}

func NewTransferState(config TransferStateConfig) (TransferState, error) {
	// establishing an initial state
	transferState := TransferState{
		State: NewState(uuid.NewV4(), true),
	}

	inStoreClient, err := store.NewClient(config.InProjectId)
	if err != nil {
		return TransferState{}, err
	}
	transferState.InStoreClient = inStoreClient
	transferState.InTransferBase = store.NewTransferBase(inStoreClient, "us-central1", config.InBucketName)

	inBucket, err := transferState.InTransferBase.GetFirmBucket()
	if err != nil {
		return TransferState{}, err
	}
	transferState.InBucket = inBucket

	outStoreClient, err := store.NewClient(config.OutProjectId)
	if err != nil {
		return TransferState{}, err
	}
	transferState.OutStoreClient = outStoreClient
	transferState.OutTransferBase = store.NewTransferBase(outStoreClient, "us-central1", config.OutBucketName)

	outBucket, err := transferState.OutTransferBase.GetFirmBucket()
	if err != nil {
		return TransferState{}, err
	}
	transferState.OutBucket = outBucket

	return transferState, nil
}

type TransferState struct {
	State

	InStoreClient  store.Client
	InTransferBase store.TransferBase
	InBucket       *storage.BucketHandle

	OutStoreClient  store.Client
	OutTransferBase store.TransferBase
	OutBucket       *storage.BucketHandle
}

func GetDestinationObjectName(name string) string {
	//return fmt.Sprintf("%s/%s", gameversions.Retail, name)
	return name
}

func (transferState TransferState) Copy(name string) (bool, error) {
	src, err := transferState.InTransferBase.GetFirmObject(name, transferState.InBucket)
	if err != nil {
		return false, err
	}

	destinationName := GetDestinationObjectName(name)

	dst := transferState.OutTransferBase.GetObject(destinationName, transferState.OutBucket)
	destinationExists, err := transferState.OutTransferBase.ObjectExists(dst)
	if err != nil {
		return false, err
	}
	if destinationExists {
		logging.WithField("object", destinationName).Info("Object exists")

		return false, nil
	}

	copier := dst.CopierFrom(src)
	if _, err := copier.Run(transferState.OutStoreClient.Context); err != nil {
		return false, err
	}

	return true, nil
}

func (transferState TransferState) DeleteAtDestination(name string) (bool, error) {
	destinationName := GetDestinationObjectName(name)

	dst := transferState.OutTransferBase.GetObject(destinationName, transferState.OutBucket)
	destinationExists, err := transferState.OutTransferBase.ObjectExists(dst)
	if err != nil {
		return false, err
	}
	if !destinationExists {
		logging.WithField("destination-name", destinationName).Info("Destination object does not exist")

		return false, nil
	}

	if err := dst.Delete(transferState.OutStoreClient.Context); err != nil {
		return false, err
	}

	return true, nil
}

func (transferState TransferState) DeleteAtSource(name string) (bool, error) {
	src := transferState.InTransferBase.GetObject(name, transferState.InBucket)
	sourceExists, err := transferState.InTransferBase.ObjectExists(src)
	if err != nil {
		return false, err
	}
	if !sourceExists {
		logging.WithField("source-name", name).Info("Source object does not exist")

		return false, nil
	}

	if err := src.Delete(transferState.InStoreClient.Context); err != nil {
		return false, err
	}

	return true, nil
}

type RunJob struct {
	Err     error
	Name    string
	Deleted bool
}

func (transferState TransferState) Run() error {
	// spawning workers
	in := make(chan string)
	out := make(chan RunJob)
	worker := func() {
		for name := range in {
			deleted, err := transferState.DeleteAtSource(name)
			if err != nil {
				out <- RunJob{
					Err:     err,
					Name:    name,
					Deleted: false,
				}

				continue
			}

			out <- RunJob{
				Err:     nil,
				Name:    name,
				Deleted: deleted,
			}
		}
	}
	postWork := func() {
		close(out)
	}
	util.Work(16, worker, postWork)

	// spinning it up
	go func() {
		it := transferState.InBucket.Objects(transferState.InStoreClient.Context, nil)
		for {
			objAttrs, err := it.Next()
			if err != nil {
				if err == iterator.Done {
					break
				}

				logging.WithField("error", err.Error()).Fatal("Failed to iterate to next")

				continue
			}

			logging.WithField("name", objAttrs.Name).Info("Found object, enqueueing")
			in <- objAttrs.Name
		}

		close(in)
	}()

	// waiting for the results to drain out
	total := 0
	for job := range out {
		if job.Err != nil {
			return job.Err
		}

		if job.Deleted {
			logging.WithField("name", job.Name).Info("Deleted object at source")

			total++
		}
	}

	logging.WithField("total", total).Info("Copied objects to destination")

	return nil
}
