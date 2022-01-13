package services

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/paulosotu/local-storage-sync/pkg/models"

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

type SyncConfig interface {
	GetTimerTick() int
	GetNodeName() string
	GetDataDir() string
}

//.GetTimerTick, storageService, config.NodeName, config.DataDir
func NewRSyncRunner(cfg SyncConfig, storage StorageLocationService) *RSyncRunner {
	return &RSyncRunner{
		ticker:          time.NewTicker(time.Duration(cfg.GetTimerTick()) * time.Second),
		storageLocation: storage,
		currentNodeName: cfg.GetNodeName(),
		rootDataPath:    cfg.GetDataDir(),
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
					log.Errorf("Failed to get deamon set nodes with error %v", err)
					continue
				}
				locationsToSync := make([]models.StoragePodLocation, 0, len(storageLocations))
				nodesToSync := make([]models.Node, 0, len(storageLocations))
				for _, stLocation := range storageLocations {
					if stLocation.GetNodeName() == r.currentNodeName {
						locationsToSync = append(locationsToSync, stLocation)
					}
				}
				for _, node := range nodes {
					if node.GetName() != r.currentNodeName {
						nodesToSync = append(nodesToSync, node)
					}
				}
				log.Debugf("nodes: %v", nodes)
				log.Debugf("Nodes to Sync: %v", nodesToSync)
				log.Debugf("locationsToSync to Sync: %v", locationsToSync)
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
			arg3 := "--delete"

			log.Infof("[RSyncRunner] Running command: %s %s %s %s", rsync, arg0, arg1, arg2, arg3)
			cmd := exec.Command(rsync, arg0, arg1, arg2, arg3)
			stdout, err := cmd.Output()

			if err != nil {
				log.Errorf("Failed to run rsync command with error %v", err)
				//log.Fatal(err.Error())
				return
			}

			log.Infof("[RSyncRunner] Starting Command output:")
			// Print the output
			fmt.Println(string(stdout))
			log.Infof("[RSyncRunner] Finished Command output")
		}

	}
}
