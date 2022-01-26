package services

import (
	"bufio"
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/paulosotu/local-storage-sync/pkg/models"
	"github.com/paulosotu/local-storage-sync/pkg/services/mocks"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/cache"

	"github.com/stretchr/testify/assert"
)

//go:generate mockery --dir ../../vendor/k8s.io/client-go/informers --name SharedInformerFactory
//go:generate mockery --dir ../../vendor/k8s.io/client-go/informers --name SharedInformerFactory
//go:generate mockery --dir ../../vendor/k8s.io/client-go/tools/cache --name InformerSynced
//go:generate mockery --dir ../../vendor/k8s.io/client-go/listers/core/v1 --name PodLister
//go:generate mockery --dir ../../vendor/k8s.io/client-go/listers/core/v1 --name PersistentVolumeClaimLister
//go:generate mockery --dir ../../vendor/k8s.io/apimachinery/pkg/labels --name Selector

func mockLogger(t *testing.T) (*bufio.Scanner, *os.File, *os.File) {
	reader, writer, err := os.Pipe()
	if err != nil {
		assert.Fail(t, "couldn't get os Pipe: %v", err)
	}
	log.SetOutput(writer)

	return bufio.NewScanner(reader), reader, writer
}

func resetLogger(reader *os.File, writer *os.File) {
	reader.Close()
	writer.Close()
	log.SetOutput(os.Stdout)
}

func parseLog(text string) map[string]string {
	re := regexp.MustCompile(`[a-zA-Z]+=(\"([^"]+\s*)+\")|(([^"]+)+\s)`)

	submatch := re.FindAllString(text, -1)
	ret := make(map[string]string)

	for _, element := range submatch {
		values := strings.Split(element, "=")
		ret[strings.TrimSpace(values[0])] = strings.Trim(strings.TrimSpace(values[1]), "\"")
	}
	return ret
}

func TestStart(t *testing.T) {
	testCases := []struct {
		serviceStarted    bool
		cacheToSyncReturn bool
		testFunc          func(*testing.T, *KubeCorePVCService, *mocks.SharedInformerFactory, *mocks.SharedInformerFactory)
		description       string
	}{
		{
			description:       "Already started service must log message that is already running.",
			serviceStarted:    true,
			cacheToSyncReturn: false,
			testFunc: func(t *testing.T,
				coreService *KubeCorePVCService,
				podFactory *mocks.SharedInformerFactory,
				mockPvcFactory *mocks.SharedInformerFactory) {
				t.Helper()
				scanner, reader, writer := mockLogger(t)
				defer resetLogger(reader, writer)
				coreService.Start()
				scanner.Scan()
				got := scanner.Text()
				msg := "KubeCorePVCService already started..."
				assert.Contains(t, got, msg)
			},
		}, {
			description:       "Must call Start of pod factory and pvcfactory.",
			serviceStarted:    false,
			cacheToSyncReturn: false,
			testFunc: func(t *testing.T,
				coreService *KubeCorePVCService,
				podFactory *mocks.SharedInformerFactory,
				mockPvcFactory *mocks.SharedInformerFactory) {
				t.Helper()
				coreService.Start()

				var stopRO <-chan struct{} = coreService.stopch
				mockPvcFactory.AssertCalled(t, "Start", stopRO)
				podFactory.AssertCalled(t, "Start", stopRO)
			},
		}, {
			description:       "Must stop and log error if failed to wait for cache to sync.",
			serviceStarted:    false,
			cacheToSyncReturn: false,
			testFunc: func(t *testing.T,
				coreService *KubeCorePVCService,
				podFactory *mocks.SharedInformerFactory,
				mockPvcFactory *mocks.SharedInformerFactory) {
				t.Helper()
				scanner, reader, writer := mockLogger(t)
				defer resetLogger(reader, writer)
				coreService.Start()

				receivedShutdown, receivedStop := false, false

				for run := true; run; {
					select {
					case _, ok := <-coreService.stopch:
						if !ok {
							coreService.stopch = nil
						}
						receivedStop = true
					case _, ok := <-coreService.shutdownch:
						if !ok {
							coreService.shutdownch = nil
						}
						receivedShutdown = true

					case <-time.After(time.Millisecond * 50):
						run = false
					}
				}

				writer.Close()
				validatedError := false
				for scanner.Scan() {
					got := scanner.Text()
					l := parseLog(got)

					if l["level"] == "error" {
						validatedError = true
						assert.Contains(t, l["msg"], "failed to wait for caches to sync")
						break
					}
				}
				assert.True(t, validatedError, "Expecting log error message \"failed to wait for caches to sync\" and no error message found!")
				assert.True(t, receivedShutdown && receivedStop, "Expecting stop(%v) and shutdown(%v) channels to be triggered in under 50ms!", receivedStop, receivedShutdown)
				assert.False(t, coreService.started, "Expecting not to been started yet.")
			},
		}, {
			description:       "Ready channel must be triggered if sync cache is successfull and started must be true.",
			serviceStarted:    false,
			cacheToSyncReturn: true,
			testFunc: func(t *testing.T,
				coreService *KubeCorePVCService,
				podFactory *mocks.SharedInformerFactory,
				mockPvcFactory *mocks.SharedInformerFactory) {

				ready := false
				coreService.Start()
				for run := true; run; {
					select {
					case <-coreService.readych:
						ready = true
						run = false
					case <-time.After(time.Millisecond * 50):
						run = false
					}
				}
				assert.True(t, ready, "Expecting ready channel to be triggered after a successful cache sync")
				assert.True(t, coreService.started, "Expecting core service to be in a started state after a successful cache sync")

				close(coreService.stopch)
				for run := true; run; {
					select {
					case <-coreService.shutdownch:
						run = false
					case <-time.After(time.Millisecond * 50):
						run = false
					}
				}
				assert.False(t, coreService.started, "Expecting core service to be in a stopped state after closing the stop channel")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockPvcFactory := &mocks.SharedInformerFactory{}
			podFactory := &mocks.SharedInformerFactory{}
			coreService := &KubeCorePVCService{
				started:        tc.serviceStarted,
				pvcFactory:     mockPvcFactory,
				podNodeFactory: podFactory,
				stopch:         make(chan struct{}),
				readych:        make(chan struct{}),
				shutdownch:     make(chan struct{}),
				waitForCacheToSync: func(controllerName string, stopCh <-chan struct{}, cacheSyncs ...cache.InformerSynced) bool {
					return tc.cacheToSyncReturn
				},
			}
			var stopRO <-chan struct{} = coreService.stopch
			mockPvcFactory.On("Start", stopRO).Times(1).Return()
			podFactory.On("Start", stopRO).Times(1).Return()
			tc.testFunc(t, coreService, podFactory, mockPvcFactory)

		})
	}
}

func TestWaitForReady(t *testing.T) {
	type listStruct struct {
		coreService *KubeCorePVCService
		context     context.Context
		cancel      func()
	}
	coreServiceList := make([]listStruct, 0, 6)
	for i := 0; i < 6; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		coreServiceList = append(coreServiceList, listStruct{
			coreService: &KubeCorePVCService{
				stopch:  make(chan struct{}),
				readych: make(chan struct{}),
			},
			context: ctx,
			cancel:  cancel,
		})
	}

	testCases := []struct {
		expected       bool
		channel        <-chan struct{}
		service        *KubeCorePVCService
		context        context.Context
		triggerChannel func()
		description    string
	}{
		{
			description: "If ready channel message than returns true.",
			expected:    true,
			service:     coreServiceList[0].coreService,
			context:     coreServiceList[0].context,
			channel:     coreServiceList[0].coreService.readych,
			triggerChannel: func() {
				coreServiceList[0].coreService.readych <- struct{}{}
			},
		}, {
			description: "If ready channel close than returns true.",
			expected:    true,
			service:     coreServiceList[1].coreService,
			context:     coreServiceList[1].context,
			channel:     coreServiceList[1].coreService.readych,
			triggerChannel: func() {
				close(coreServiceList[1].coreService.readych)
			},
		}, {
			description:    "If context cancelled than returns false.",
			expected:       false,
			service:        coreServiceList[2].coreService,
			context:        coreServiceList[2].context,
			channel:        coreServiceList[2].context.Done(),
			triggerChannel: coreServiceList[2].cancel,
		}, {
			description: "If stop channel message than returns false.",
			expected:    false,
			service:     coreServiceList[3].coreService,
			context:     coreServiceList[3].context,
			channel:     coreServiceList[3].coreService.stopch,
			triggerChannel: func() {
				coreServiceList[3].coreService.stopch <- struct{}{}
			},
		}, {
			description: "If stop channel closed than returns false.",
			expected:    false,
			service:     coreServiceList[4].coreService,
			context:     coreServiceList[4].context,
			channel:     coreServiceList[4].coreService.stopch,
			triggerChannel: func() {
				close(coreServiceList[4].coreService.stopch)
			},
		}, {
			description:    "If no trigger should block.",
			expected:       false,
			service:        coreServiceList[5].coreService,
			context:        coreServiceList[5].context,
			channel:        nil,
			triggerChannel: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			finished := make(chan bool)
			if tc.triggerChannel != nil {
				go func() {
					go tc.triggerChannel()
					finished <- tc.service.WaitForReady(tc.context)
				}()
				ret := false
				for run := true; run; {
					select {
					case ret = <-finished:
						run = false
					case <-time.After(time.Millisecond * 50):
						run = false
						assert.FailNow(t, "No return received after 50ms, would block!?")
					}
				}
				assert.EqualValues(t, tc.expected, ret, "Expecting %v and got %v", tc.expected, ret)
			} else {
				go func() {
					finished <- tc.service.WaitForReady(tc.context)
				}()

				for run := true; run; {
					select {
					case <-finished:
						run = false
						assert.FailNow(t, "Expecting to block but received a return")
					case <-time.After(time.Millisecond * 50):
						run = false
						assert.True(t, true, "Considered blocked after 50ms timeout")
					}
				}
			}
		})
	}
}

func TestGetStorageLocations(t *testing.T) {
	mockSelector := &mocks.Selector{}
	validateLabelRequirements := func(t *testing.T, s1 string, o selection.Operator, s2 []string, po ...field.PathOption) {
		t.Helper()
		if s1 == "syncronize-nodes" {
			assert.EqualValues(t, selection.Equals, o)
			assert.EqualValues(t, []string{"true"}, s2)
		} else if s1 == "app" {
			assert.EqualValues(t, selection.In, o)
			assert.EqualValues(t, []string{"test"}, s2)
		} else {
			assert.FailNow(t, "Unknown Label requirement %v", s1)
		}
	}
	mockSuccessPVList := func(t *testing.T, claimLister *mocks.PersistentVolumeClaimLister) {
		pvList := []*corev1.PersistentVolumeClaim{
			{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
					Name:      "PV_NAME",
					Namespace: "NAMESPACE",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "VolumeName1",
				},
			}, {
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
					Name:      "PV_NAME2",
					Namespace: "NAMESPACE",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "VolumeName2",
				},
			},
		}

		claimLister.On("List", mockSelector).Times(1).Return(pvList, nil)
	}
	testCases := []struct {
		expectedError string
		expected      []models.StoragePodLocation
		service       *KubeCorePVCService
		description   string
		setupMocks    func(claimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister)
	}{
		{
			description:   "If failed to create syncronize-nodes requirement an error should be returned",
			expectedError: "SYNC_NODE_REQUIREMENT_SELECTOR_ERROR",
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					return nil, errors.New("SYNC_NODE_REQUIREMENT_SELECTOR_ERROR")
				},
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
			},
			setupMocks: func(claimLister *mocks.PersistentVolumeClaimLister,
				podLister *mocks.PodLister) {
			},
		}, {
			description:   "If an error occours in list pvcs",
			expectedError: "LIST_PVCS_ERROR",
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return labels.NewRequirement(s1, o, s2, po...)
				},
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(claimLister *mocks.PersistentVolumeClaimLister,
				podLister *mocks.PodLister) {
				t.Helper()
				claimLister.On("List", mockSelector).Times(1).Return(nil, errors.New("LIST_PVCS_ERROR"))
			},
		},
		{
			description:   "If no pvcs are available",
			expectedError: "no persistent volume claims available",
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return labels.NewRequirement(s1, o, s2, po...)
				},
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(claimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister) {
				t.Helper()
				claimLister.On("List", mockSelector).Times(1).Return(make([]*corev1.PersistentVolumeClaim, 0), nil)
			},
		},
		{
			description:   "If failed to create app requirement an error should be returned",
			expectedError: "APP_REQUIREMENT_SELECTOR_ERROR",
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return nil, errors.New("APP_REQUIREMENT_SELECTOR_ERROR")
				},
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(claimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister) {
				t.Helper()
				mockSuccessPVList(t, claimLister)
			},
		}, {
			description:   "If an error occours listing pods",
			expectedError: "POD_LIST_ERROR",
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return labels.NewRequirement(s1, o, s2, po...)
				},
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(pvClaimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister) {
				t.Helper()
				mockSuccessPVList(t, pvClaimLister)
				podLister.On("List", mockSelector).Times(1).Return(nil, errors.New("POD_LIST_ERROR"))
			},
		}, {
			description: "If an empty list of pods is returned",
			expected:    []models.StoragePodLocation{},
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return labels.NewRequirement(s1, o, s2, po...)
				},
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(pvClaimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister) {
				t.Helper()
				mockSuccessPVList(t, pvClaimLister)
				podLister.On("List", mockSelector).Times(1).Return([]*corev1.Pod{}, nil)
			},
		}, {
			description: "List of expected Pod locations is returned",
			expected: []models.StoragePodLocation{
				*models.NewStoragePodLocation("Node1", "127.0.0.1", "PV_NAME", "NAMESPACE", "pod1", "127.0.0.1", "VolumeName1"),
				*models.NewStoragePodLocation("Node1", "127.0.0.1", "PV_NAME2", "NAMESPACE", "pod2", "127.0.0.1", "VolumeName2"),
			},
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return labels.NewRequirement(s1, o, s2, po...)
				},
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(pvClaimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister) {
				t.Helper()
				mockSuccessPVList(t, pvClaimLister)
				podList := []*corev1.Pod{
					{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "PV_NAME",
										},
									},
								},
							},
							NodeName: "Node1",
						},
						Status: corev1.PodStatus{
							HostIP: "127.0.0.1",
							PodIP:  "127.0.0.1",
						},
						ObjectMeta: v1.ObjectMeta{
							Name: "pod1",
						},
					}, {
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "PV_NAME2",
										},
									},
								},
							},
							NodeName: "Node1",
						},
						Status: corev1.PodStatus{
							HostIP: "127.0.0.1",
							PodIP:  "127.0.0.1",
						},
						ObjectMeta: v1.ObjectMeta{
							Name: "pod2",
						},
					},
				}
				podLister.On("List", mockSelector).Times(1).Return(podList, nil)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tc.setupMocks(tc.service.pvClaimLister.(*mocks.PersistentVolumeClaimLister),
				tc.service.podLister.(*mocks.PodLister))
			list, err := tc.service.GetStorageLocations()
			if err != nil {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.ElementsMatch(t, list, tc.expected)
			}
		})
	}
}

func TestGetNodes(t *testing.T) {
	mockSelector := &mocks.Selector{}

	validateLabelRequirements := func(t *testing.T, s1 string, o selection.Operator, s2 []string, po ...field.PathOption) {
		t.Helper()
		assert.EqualValues(t, s1, "app")
		assert.EqualValues(t, selection.In, o)
		assert.EqualValues(t, []string{"APP_NAME"}, s2)
	}

	testCases := []struct {
		expectedError string
		expected      []models.Node
		service       *KubeCorePVCService
		description   string
		setupMocks    func(claimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister)
	}{
		{
			description:   "If failed to create app requirement an error should be returned",
			expectedError: "APP_REQUIREMENT_SELECTOR_ERROR",
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return nil, errors.New("APP_REQUIREMENT_SELECTOR_ERROR")
				},
				config:        models.NewConfig(models.WithAppName("APP_NAME")),
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(claimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister) {
				t.Helper()
			},
		},
		{
			description:   "If failed to list pods an error should be returned",
			expectedError: "LIST_POD_ERROR",
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return labels.NewRequirement(s1, o, s2, po...)
				},
				config:        models.NewConfig(models.WithAppName("APP_NAME")),
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(claimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister) {
				t.Helper()
				podLister.On("List", mockSelector).Times(1).Return(nil, errors.New("LIST_POD_ERROR"))
			},
		}, {
			description:   "If an empty list of pods is returned",
			expectedError: "",
			expected:      []models.Node{},
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return labels.NewRequirement(s1, o, s2, po...)
				},
				config:        models.NewConfig(models.WithAppName("APP_NAME")),
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(claimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister) {
				t.Helper()
				podLister.On("List", mockSelector).Times(1).Return([]*corev1.Pod{}, nil)
			},
		}, {
			description:   "List of expected Nodes is returned",
			expectedError: "",
			expected: []models.Node{
				*models.NewNode("Node1", "127.0.0.1", "ok"),
				*models.NewNode("Node2", "127.0.0.2", "down"),
			},
			service: &KubeCorePVCService{
				newLabelRequirement: func(s1 string, o selection.Operator, s2 []string, po ...field.PathOption) (*labels.Requirement, error) {
					validateLabelRequirements(t, s1, o, s2, po...)
					return labels.NewRequirement(s1, o, s2, po...)
				},
				config:        models.NewConfig(models.WithAppName("APP_NAME")),
				pvClaimLister: &mocks.PersistentVolumeClaimLister{},
				podLister:     &mocks.PodLister{},
				addLabelRequirement: func(r *labels.Requirement) labels.Selector {
					validateLabelRequirements(t, r.Key(), r.Operator(), r.Values().List(), nil)
					return mockSelector
				},
			},
			setupMocks: func(claimLister *mocks.PersistentVolumeClaimLister, podLister *mocks.PodLister) {
				t.Helper()
				podList := []*corev1.Pod{
					{
						Spec: corev1.PodSpec{
							NodeName: "Node1",
						},
						Status: corev1.PodStatus{
							PodIP: "127.0.0.1",
							Phase: "ok",
						},
					}, {
						Spec: corev1.PodSpec{
							NodeName: "Node2",
						},
						Status: corev1.PodStatus{
							PodIP: "127.0.0.2",
							Phase: "down",
						},
					},
				}
				podLister.On("List", mockSelector).Times(1).Return(podList, nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tc.setupMocks(tc.service.pvClaimLister.(*mocks.PersistentVolumeClaimLister),
				tc.service.podLister.(*mocks.PodLister))
			list, err := tc.service.GetNodes()
			if err != nil {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.ElementsMatch(t, list, tc.expected)
			}
		})
	}
}
func TestStop(t *testing.T) {
	testCases := []struct {
		serviceStarted    bool
		cacheToSyncReturn bool
		testFunc          func(*testing.T, *KubeCorePVCService)
		description       string
	}{
		{
			description:       "Not Started service should not send any stop signal.",
			serviceStarted:    false,
			cacheToSyncReturn: false,
			testFunc: func(t *testing.T,
				coreService *KubeCorePVCService) {
				t.Helper()
				receivedStop := false
				go coreService.Stop()
				for run := true; run; {
					select {
					case _, ok := <-coreService.stopch:
						if !ok {
							coreService.stopch = nil
						}
						receivedStop = true
						run = false
					case <-time.After(time.Millisecond * 50):
						run = false
					}
				}
				assert.False(t, receivedStop, "Should not receive a stop with a 50ms timeout")
			},
		}, {
			description:       "Should send stop signal and block.",
			serviceStarted:    true,
			cacheToSyncReturn: false,
			testFunc: func(t *testing.T,
				coreService *KubeCorePVCService) {
				t.Helper()
				receivedStop := false
				timeout := false
				stopCallFinish := make(chan struct{})
				go func() {
					coreService.Stop()
					stopCallFinish <- struct{}{}
				}()
				for run := true; run; {
					select {
					case _, ok := <-coreService.stopch:
						if !ok {
							coreService.stopch = nil
						}
						receivedStop = true
					case <-stopCallFinish:
						run = false
					case <-time.After(time.Millisecond * 50):
						run = false
						timeout = true
					}
				}
				assert.True(t, timeout && receivedStop, "Should receive a stop message (%v) but stop call must block (%v).", receivedStop, timeout)
			},
		}, {
			description:       "Should send stop signal and unblock after receiving a shutdown.",
			serviceStarted:    true,
			cacheToSyncReturn: false,
			testFunc: func(t *testing.T,
				coreService *KubeCorePVCService) {
				t.Helper()
				timeout := false
				receivedStop := false
				stopCallFinish := make(chan struct{})
				go func() {
					coreService.Stop()
					stopCallFinish <- struct{}{}
				}()
				for run := true; run; {
					select {
					case _, ok := <-coreService.stopch:
						if !ok {
							coreService.stopch = nil
						}
						receivedStop = true
						go func() {
							coreService.shutdownch <- struct{}{}
						}()

					case <-stopCallFinish:
						run = false
					case <-time.After(time.Millisecond * 50):
						run = false
						timeout = true
					}
				}
				assert.True(t, receivedStop)
				assert.False(t, timeout, "Cannot block since receives a shutdown after sending stop message.")
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			coreService := &KubeCorePVCService{
				started:    tc.serviceStarted,
				stopch:     make(chan struct{}),
				readych:    make(chan struct{}),
				shutdownch: make(chan struct{}),
			}
			tc.testFunc(t, coreService)

		})
	}
}
