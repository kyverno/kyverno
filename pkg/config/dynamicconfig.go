package config

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/minio/minio/pkg/wildcard"
	v1 "k8s.io/api/core/v1"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// read the conifgMap with name in env:INIT_CONFIG
// this configmap stores the resources that are to be filtered
const cmNameEnv string = "INIT_CONFIG"
const cmDataField string = "resourceFilters"

type ConfigData struct {
	client kubernetes.Interface
	// configMap Name
	cmName string
	// lock configuration
	mux sync.RWMutex
	// configuration data
	filters []k8Resource
	// hasynced
	cmListerSycned cache.InformerSynced
}

// ToFilter checks if the given resource is set to be filtered in the configuration
func (cd *ConfigData) ToFilter(kind, namespace, name string) bool {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	for _, f := range cd.filters {
		if wildcard.Match(f.Kind, kind) && wildcard.Match(f.Namespace, namespace) && wildcard.Match(f.Name, name) {
			return true
		}
	}
	return false
}

// Interface to be used by consumer to check filters
type Interface interface {
	ToFilter(kind, namespace, name string) bool
}

// NewConfigData ...
func NewConfigData(rclient kubernetes.Interface, cmInformer informers.ConfigMapInformer, filterK8Resources string) *ConfigData {
	// environment var is read at start only
	if cmNameEnv == "" {
		glog.Info("ConfigMap name not defined in env:INIT_CONFIG: loading no default configuration")
	}
	cd := ConfigData{
		client:         rclient,
		cmName:         os.Getenv(cmNameEnv),
		cmListerSycned: cmInformer.Informer().HasSynced,
	}
	//TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	if filterK8Resources != "" {
		glog.Info("Init configuration from commandline arguments")
		cd.initFilters(filterK8Resources)
	}

	cmInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cd.addCM,
		UpdateFunc: cd.updateCM,
		DeleteFunc: cd.deleteCM,
	})
	return &cd
}

//Run checks syncing
func (cd *ConfigData) Run(stopCh <-chan struct{}) {
	// wait for cache to populate first time
	if !cache.WaitForCacheSync(stopCh, cd.cmListerSycned) {
		glog.Error("configuration: failed to sync informer cache")
	}
}

func (cd *ConfigData) addCM(obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if cm.Name != cd.cmName {
		return
	}
	cd.load(*cm)
	// else load the configuration
}

func (cd *ConfigData) updateCM(old, cur interface{}) {
	cm := cur.(*v1.ConfigMap)
	if cm.Name != cd.cmName {
		return
	}
	// if data has not changed then dont load configmap
	cd.load(*cm)
}

func (cd *ConfigData) deleteCM(obj interface{}) {
	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		_, ok = tombstone.Obj.(*v1.ConfigMap)
		if !ok {
			glog.Info(fmt.Errorf("Tombstone contained object that is not a ConfigMap %#v", obj))
			return
		}
	}

	if cm.Name != cd.cmName {
		return
	}
	// remove the configuration paramaters
	cd.unload(*cm)
}

func (cd *ConfigData) load(cm v1.ConfigMap) {
	if cm.Data == nil {
		glog.V(4).Infof("Configuration: No data defined in ConfigMap %s", cm.Name)
		return
	}
	// get resource filters
	filters, ok := cm.Data["resourceFilters"]
	if !ok {
		glog.V(4).Infof("Configuration: No resourceFilters defined in ConfigMap %s", cm.Name)
		return
	}
	// filters is a string
	if filters == "" {
		glog.V(4).Infof("Configuration: resourceFilters is empty in ConfigMap %s", cm.Name)
		return
	}
	// parse and load the configuration
	cd.mux.Lock()
	defer cd.mux.Unlock()

	newFilters := parseKinds(filters)
	if reflect.DeepEqual(newFilters, cd.filters) {
		glog.V(4).Infof("Configuration: resourceFilters did not change in ConfigMap %s", cm.Name)
		return
	}
	glog.V(4).Infof("Configuration: Old resource filters %v", cd.filters)
	glog.Infof("Configuration: New resource filters to %v", newFilters)
	// update filters
	cd.filters = newFilters
}

//TODO: this has been added to backward support command line arguments
// will be removed in future and the configuration will be set only via configmaps
func (cd *ConfigData) initFilters(filters string) {
	// parse and load the configuration
	cd.mux.Lock()
	defer cd.mux.Unlock()

	newFilters := parseKinds(filters)
	glog.Infof("Configuration: Init resource filters to %v", newFilters)
	// update filters
	cd.filters = newFilters
}

func (cd *ConfigData) unload(cm v1.ConfigMap) {
	// TODO pick one msg
	glog.Infof("Configuration: ConfigMap %s deleted, removing configuration filters", cm.Name)
	glog.Infof("Configuration: Removing all resource filters as ConfigMap %s deleted", cm.Name)
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.filters = []k8Resource{}
}

type k8Resource struct {
	Kind      string //TODO: as we currently only support one GVK version, we use the kind only. But if we support multiple GVK, then GV need to be added
	Namespace string
	Name      string
}

//ParseKinds parses the kinds if a single string contains comma seperated kinds
// {"1,2,3","4","5"} => {"1","2","3","4","5"}
func parseKinds(list string) []k8Resource {
	resources := []k8Resource{}
	var resource k8Resource
	re := regexp.MustCompile(`\[([^\[\]]*)\]`)
	submatchall := re.FindAllString(list, -1)
	for _, element := range submatchall {
		element = strings.Trim(element, "[")
		element = strings.Trim(element, "]")
		elements := strings.Split(element, ",")
		//TODO: wildcards for namespace and name
		if len(elements) == 0 {
			continue
		}
		if len(elements) == 3 {
			resource = k8Resource{Kind: elements[0], Namespace: elements[1], Name: elements[2]}
		}
		if len(elements) == 2 {
			resource = k8Resource{Kind: elements[0], Namespace: elements[1]}
		}
		if len(elements) == 1 {
			resource = k8Resource{Kind: elements[0]}
		}
		resources = append(resources, resource)
	}
	return resources
}
