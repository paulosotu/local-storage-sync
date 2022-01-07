package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/paulosotu/local-storage-sync/models"
	"github.com/paulosotu/local-storage-sync/utils"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const (
	refresh_time = 3
)

type KubeCoreConfig interface {
	GetDSAppName() string
	GetSelector() string
	ShouldRunInCluster() bool
}

type KubeCorePVCService struct {
	pvcFactory     informers.SharedInformerFactory
	podNodeFactory informers.SharedInformerFactory

	pvClaimLister corelisters.PersistentVolumeClaimLister
	podLister     corelisters.PodLister

	pvClaimSynced cache.InformerSynced
	podSynced     cache.InformerSynced

	config KubeCoreConfig

	started bool

	stopch  chan struct{}
	readych chan struct{}

	shutdownch chan struct{}
}

func NewKubeCorePVCService(cfg KubeCoreConfig) *KubeCorePVCService {
	var err error
	var config *rest.Config

	if cfg.ShouldRunInCluster() {
		log.Debug("Should run inside the cluster")
		config, err = rest.InClusterConfig()
	} else {
		log.Debug("Should run outside the cluster")
		kubeconfig := filepath.Join(
			os.Getenv("HOME"), ".kube", "config",
		)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		log.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	pvcFactory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		time.Second*refresh_time,
		informers.WithTweakListOptions(
			func(opt *metav1.ListOptions) {
				opt.LabelSelector = cfg.GetSelector()
			},
		),
	)

	podFactory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		time.Second*refresh_time,
	)

	ret := &KubeCorePVCService{
		pvcFactory:     pvcFactory,
		podNodeFactory: podFactory,
		started:        false,
	}

	informerPVC := pvcFactory.Core().V1().PersistentVolumeClaims()
	informerPod := podFactory.Core().V1().Pods()

	ret.pvClaimLister = informerPVC.Lister()
	ret.pvClaimSynced = informerPVC.Informer().HasSynced

	ret.podLister = informerPod.Lister()
	ret.podSynced = informerPod.Informer().HasSynced

	ret.config = cfg

	return ret
}

func (k *KubeCorePVCService) Start() {
	if k.started {
		log.Fatal("KubeCorePVCService already started...")
	}

	k.stopch = make(chan struct{})
	k.readych = make(chan struct{})
	k.shutdownch = make(chan struct{})

	defer runtime.HandleCrash()
	log.Info("starting KubeCorePVCService")

	k.podNodeFactory.Start(k.stopch)
	k.pvcFactory.Start(k.stopch)

	k.started = true

	go func() {
		if ok := cache.WaitForCacheSync(k.stopch, k.pvClaimSynced, k.podSynced); !ok {
			k.Stop()
			log.Error("failed to wait for caches to sync")
			return
		}

		close(k.readych)

		_, k.started = <-k.stopch
		log.Info("[KubeCorePVCService] service terminated")
		close(k.shutdownch)
	}()
}

func (k *KubeCorePVCService) WaitForReady(ctx context.Context) bool {
	select {
	case <-k.readych:
		return true
	case <-k.stopch:
		return false
	case <-ctx.Done():
		return false
	}
}

func (k *KubeCorePVCService) GetStorageLocations() ([]models.StoragePodLocation, error) {
	var statefulsets []*corev1.Pod

	req, err := labels.NewRequirement("syncronize-nodes", selection.Equals, []string{"true"})
	if err != nil {
		return nil, err
	}
	pvcs, err := k.pvClaimLister.List(labels.NewSelector().Add(*req))
	// var req *labels.Requirement
	// pvcs, err := k.pvClaimLister.List(labels.Everything())
	if err != nil {
		return nil, err
	} else if len(pvcs) < 1 {
		return nil, errors.New("no persistent volume claims available")
	}
	pvcMap, appNameSet := k.createPVCSList(pvcs)

	if req, err = labels.NewRequirement("app", selection.In, appNameSet.GetValues()); err != nil {
		return nil, err
	}

	if statefulsets, err = k.podLister.List(labels.NewSelector().Add(*req)); err != nil {
		return nil, err
	}

	pvcToPod := k.createPodMapForPVC(statefulsets, pvcMap)

	return k.createStoragePodLocationList(pvcToPod, pvcMap), err
}

func (k *KubeCorePVCService) GetNodes() ([]models.Node, error) {
	var storageSyncPods []*corev1.Pod = nil
	var req *labels.Requirement = nil
	var err error = nil

	if req, err = labels.NewRequirement("app", selection.In, []string{k.config.GetDSAppName()}); err != nil {
		return nil, err
	}
	if storageSyncPods, err = k.podLister.List(labels.NewSelector().Add(*req)); err != nil {
		return nil, err
	}
	ret := make([]models.Node, 0, len(storageSyncPods))
	for _, pod := range storageSyncPods {
		ret = append(ret, *models.NewNode(pod.Spec.NodeName, pod.Status.PodIP, string(pod.Status.Phase)))
	}

	return ret, err
}

func (k *KubeCorePVCService) Stop() {
	if k.started {
		k.started = false
		log.Info("[KubeCorePVCService] Shutting down")
		k.stopch <- struct{}{}
		close(k.stopch)
		log.Info("[KubeCorePVCService] Waiting for shutdown to complete ...")
		<-k.shutdownch
		log.Info("[KubeCorePVCService] Shutdown completed")
	}
}

func (k *KubeCorePVCService) createStoragePodLocationList(pvcToPod map[string]corev1.Pod, pvcs map[string]corev1.PersistentVolumeClaim) []models.StoragePodLocation {
	ret := make([]models.StoragePodLocation, 0, len(pvcs))

	for _, pvc := range pvcs {
		ret = append(ret, *models.NewStoragePodLocation(
			pvcToPod[pvc.Name].Spec.NodeName,
			pvcToPod[pvc.Name].Status.HostIP,
			pvc.Name,
			pvc.Namespace,
			pvcToPod[pvc.Name].Name,
			pvcToPod[pvc.Name].Status.PodIP,
			pvc.Spec.VolumeName,
			pvc.Status,
		))
	}

	return ret
}

func (k *KubeCorePVCService) createPodMapForPVC(pods []*corev1.Pod, pvcMap map[string]corev1.PersistentVolumeClaim) map[string]corev1.Pod {
	pvcToPod := make(map[string]corev1.Pod)

	for _, pod := range pods {
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil {
				if _, ok := pvcMap[vol.PersistentVolumeClaim.ClaimName]; ok {
					pvcToPod[vol.PersistentVolumeClaim.ClaimName] = *pod
				}
			}
		}
	}
	return pvcToPod
}

func (k *KubeCorePVCService) createPVCSList(pvcs []*corev1.PersistentVolumeClaim) (map[string]corev1.PersistentVolumeClaim, utils.StringSet) {
	appNameSet := *utils.NewStringSet()
	pvcMap := make(map[string]corev1.PersistentVolumeClaim)

	for _, pvc := range pvcs {
		if val, ok := pvc.Labels["app"]; ok {
			appNameSet.Add(val)
		} else {
			continue
		}
		pvcMap[pvc.GetName()] = *pvc
	}
	return pvcMap, appNameSet
}
