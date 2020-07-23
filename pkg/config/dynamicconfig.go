package config

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/minio/minio/pkg/wildcard"
	v1 "k8s.io/api/core/v1"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// read the conifgMap with name in env:INIT_CONFIG
// this configmap stores the resources that are to be filtered
const cmNameEnv string = "INIT_CONFIG"

var ExcludeGroupRule []string
// ConfigData stores the configuration
type ConfigData struct {
	client kubernetes.Interface
	// configMap Name
	cmName string
	// lock configuration
	mux sync.RWMutex
	// configuration data
	filters []k8Resource

	// ExcludeGroup Role
	excludeGroupRole []string
	// hasynced
	cmSycned cache.InformerSynced
	log      logr.Logger
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

// ToFilter checks if the given resource is set to be filtered in the configuration
func (cd *ConfigData) GetExcludeGroupRole() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludeGroupRole
}

// Interface to be used by consumer to check filters
type Interface interface {
	ToFilter(kind, namespace, name string) bool
	GetExcludeGroupRole() []string
}

// NewConfigData ...
func NewConfigData(rclient kubernetes.Interface, cmInformer informers.ConfigMapInformer, filterK8Resources,excludeGroupRole string, log logr.Logger) *ConfigData {
	// environment var is read at start only
	if cmNameEnv == "" {
		log.Info("ConfigMap name not defined in env:INIT_CONFIG: loading no default configuration")
	}
	cd := ConfigData{
		client:   rclient,
		cmName:   os.Getenv(cmNameEnv),
		cmSycned: cmInformer.Informer().HasSynced,
		log:      log,
	}
	//TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	if filterK8Resources != "" {
		cd.log.Info("init configuration from commandline arguments for filterK8Resources")
		cd.initFilters(filterK8Resources)
	}

	if excludeGroupRole != "" {
		cd.log.Info("init configuration from commandline arguments for excludeGroupRole")
		cd.initExcludeGroup(excludeGroupRole)
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
	logger := cd.log
	// wait for cache to populate first time
	if !cache.WaitForCacheSync(stopCh, cd.cmSycned) {
		logger.Info("configuration: failed to sync informer cache")
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
	logger := cd.log
	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("failed to get object from tombstone")
			return
		}
		_, ok = tombstone.Obj.(*v1.ConfigMap)
		if !ok {
			logger.Info("Tombstone contained object that is not a ConfigMap", "object", obj)
			return
		}
	}

	if cm.Name != cd.cmName {
		return
	}
	// remove the configuration parameters
	cd.unload(*cm)
}

func (cd *ConfigData) load(cm v1.ConfigMap) {
	logger := cd.log.WithValues("name", cm.Name, "namespace", cm.Namespace)
	if cm.Data == nil {
		logger.V(4).Info("configuration: No data defined in ConfigMap")
		return
	}
	// get resource filters
	filters, ok := cm.Data["resourceFilters"]
	if !ok {
		logger.V(4).Info("configuration: No resourceFilters defined in ConfigMap")
		return
	}

	// get resource filters
	excludeGroupRole, ok := cm.Data["excludeGroupRole"]
	if !ok {
		logger.V(4).Info("configuration: No excludeGroupRole defined in ConfigMap")
		return
	}
	// filters is a string
	if filters == "" {
		logger.V(4).Info("configuration: resourceFilters is empty in ConfigMap")
		return
	}
	if excludeGroupRole == "" {
		logger.V(4).Info("configuration: excludeGroupRole is empty in ConfigMap")
		return
	}
	// parse and load the configuration
	cd.mux.Lock()
	defer cd.mux.Unlock()

	newFilters := parseKinds(filters)
	if reflect.DeepEqual(newFilters, cd.filters) {
		logger.V(4).Info("resourceFilters did not change")
	}else{
		logger.V(2).Info("Updated resource filters", "oldFilters", cd.filters, "newFilters", newFilters)
		// update filters
		cd.filters = newFilters
	}
	excludeGroupRoles := parseExcludeRole(excludeGroupRole)
	if reflect.DeepEqual(excludeGroupRoles, cd.excludeGroupRole) {
		logger.V(4).Info("excludeGroupRole did not change")
	}else{
		logger.V(2).Info("Updated resource excludeGroupRoles", "oldExcludeGroupRole", cd.excludeGroupRole, "newExcludeGroupRole", excludeGroupRoles)
		// update filters
		cd.excludeGroupRole  = excludeGroupRoles
		ExcludeGroupRule = cd.excludeGroupRole
	}

}

//TODO: this has been added to backward support command line arguments
// will be removed in future and the configuration will be set only via configmaps
func (cd *ConfigData) initFilters(filters string) {
	logger := cd.log
	// parse and load the configuration
	cd.mux.Lock()
	defer cd.mux.Unlock()

	newFilters := parseKinds(filters)
	logger.V(2).Info("Init resource filters", "filters", newFilters)
	// update filters
	cd.filters = newFilters
}

func (cd *ConfigData) initExcludeGroup(excludeGroupRole string) {
	logger := cd.log
	// parse and load the configuration
	cd.mux.Lock()
	defer cd.mux.Unlock()
	excludeGroupRoles := parseExcludeRole(excludeGroupRole)
	logger.V(2).Info("Init resource excludeGroupRole", "excludeGroupRole", excludeGroupRole)
	// update filters
	cd.excludeGroupRole = excludeGroupRoles
	ExcludeGroupRule = cd.excludeGroupRole
}

func (cd *ConfigData) unload(cm v1.ConfigMap) {
	logger := cd.log
	logger.Info("ConfigMap deleted, removing configuration filters", "name", cm.Name, "namespace", cm.Namespace)
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.filters = []k8Resource{}
	cd.excludeGroupRole = []string{}
	ExcludeGroupRule = cd.excludeGroupRole
}

type k8Resource struct {
	Kind      string //TODO: as we currently only support one GVK version, we use the kind only. But if we support multiple GVK, then GV need to be added
	Namespace string
	Name      string
}

//ParseKinds parses the kinds if a single string contains comma separated kinds
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

func parseExcludeRole(list string) []string {
	elements := strings.Split(list, ",")
	var parseRole []string
	for _,e := range elements {
		parseRole = append(parseRole,e)
	}
	return parseRole
}