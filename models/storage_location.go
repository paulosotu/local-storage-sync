package models

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type StoragePodLocation struct {
	nodeName    string
	nodeIp      string
	pvcName     string
	namespace   string
	bindPodName string
	pvcStatus   corev1.PersistentVolumeClaimStatus
	pvName      string
	podIp       string
}

func NewStoragePodLocation(nodeName, nodeIp, pvcName, namespace, bindPodName, podIp, pvName string, pvcStatus corev1.PersistentVolumeClaimStatus) *StoragePodLocation {
	return &StoragePodLocation{
		nodeName:    nodeName,
		nodeIp:      nodeIp,
		pvcName:     pvcName,
		namespace:   namespace,
		bindPodName: bindPodName,
		podIp:       podIp,
		pvcStatus:   pvcStatus,
		pvName:      pvName,
	}
}

func (s *StoragePodLocation) String() string {
	return fmt.Sprintf("%-32s%-45s%-28s%-22s%-22s%-8s\n", s.pvcName, s.pvName, s.bindPodName, s.nodeName, s.namespace, s.pvcStatus.Phase)
}

func (s *StoragePodLocation) GetNodeName() string {
	return s.nodeName
}

func (s *StoragePodLocation) GetNodeIp() string {
	return s.nodeIp
}

func (s *StoragePodLocation) GetPVCName() string {
	return s.pvcName
}

func (s *StoragePodLocation) GetPodIp() string {
	return s.podIp
}

func (s *StoragePodLocation) GetBindPodName() string {
	return s.bindPodName
}

func (s *StoragePodLocation) GetPVCStatus() corev1.PersistentVolumeClaimStatus {
	return s.pvcStatus
}

func (s *StoragePodLocation) GetPVName() string {
	return s.pvName
}

func (s *StoragePodLocation) GetNamespace() string {
	return s.namespace
}

func (s *StoragePodLocation) GetHostDataDirName() string {
	return fmt.Sprintf("%s-%s-%s", s.namespace, s.pvcName, s.pvName)
}
