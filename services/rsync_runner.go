package services

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/paulosotu/local-storage-sync/models"

	log "github.com/sirupsen/logrus"
)

type StorageLocationService interface {
	GetStorageLocations() ([]models.StoragePodLocation, error)
	GetNodes() ([]models.Node, error)
}

type RSyncRunner struct {
	ticker          *time.Ticker
	storageLocation StorageLocationService
	currentNodeName string
	rootDataPath    string
}

func NewRSyncRunner(tick int, storage StorageLocationService, nodeName, dataPath string) *RSyncRunner {
	return &RSyncRunner{
		ticker:          time.NewTicker(time.Duration(tick) * time.Second),
		storageLocation: storage,
		currentNodeName: nodeName,
		rootDataPath:    dataPath,
	}
}

func (r *RSyncRunner) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.ticker.C:
				storageLocations, err := r.storageLocation.GetStorageLocations()
				if err != nil {
					log.Errorf("Failed to get Storage Locations with error %v", err)
					continue
				}
				nodes, err := r.storageLocation.GetNodes()
				if err != nil {
					log.Errorf("Failed to get Storage Locations with error %v", err)
					continue
				}
				locationsToSync := make([]models.StoragePodLocation, 0, len(storageLocations))
				nodesToSync := make([]models.Node, 0, len(storageLocations))
				for _, stLocation := range storageLocations {
					fmt.Printf("stLocation: %v currentNode: %v\n", stLocation.GetNodeName(), r.currentNodeName)
					if stLocation.GetNodeName() == r.currentNodeName {
						locationsToSync = append(locationsToSync, stLocation)
					}
				}
				for _, node := range nodes {
					if node.GetName() != r.currentNodeName {
						nodesToSync = append(nodesToSync, node)
					}
				}
				fmt.Printf("Preparing to run node for locations: %v\n", locationsToSync)
				if len(locationsToSync) > 0 {
					r.runRSyncCommand(locationsToSync, nodesToSync)
				}
				continue
			}
		}
	}()
}

func (r *RSyncRunner) Stop() {
	r.ticker.Stop()
}

func (r *RSyncRunner) runRSyncCommand(syncList []models.StoragePodLocation, nodes []models.Node) {
	//rsync -avzh /root/rpmpkgs root@192.168.0.141:/root/

	root := r.rootDataPath

	if !filepath.IsAbs(r.rootDataPath) {
		root = filepath.Join("/", r.rootDataPath)
	}

	//Call this for each node that is not self
	for _, st := range syncList {
		path := root
		path = filepath.Join(path, st.GetHostDataDirName())

		for _, n := range nodes {
			rsync := "rsync"

			arg0 := "-avzh"
			arg1 := path
			arg2 := "root@" + n.GetIP() + ":" + root

			cmd := exec.Command(rsync, arg0, arg1, arg2)
			stdout, err := cmd.Output()

			if err != nil {
				log.Fatal(err.Error())
				return
			}

			// Print the output
			fmt.Println(string(stdout))
		}

	}
}
