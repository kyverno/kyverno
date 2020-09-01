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

var defaultExcludeGroupRole []string = []string{"system:serviceaccounts:kube-system", "system:nodes", "system:kube-scheduler"}

// ConfigData stores the configuration
type ConfigData struct {
	client kubernetes.Interface
	// configMap Name
	cmName string
	// lock configuration
	mux sync.RWMutex
	// configuration data
	filters []k8Resource

	// excludeGroupRole Role
	excludeGroupRole []string

	//excludeUsername exclude username
	excludeUsername []string

	//restrictDevelopmentUsername exclude dev username like minikube and kind
	restrictDevelopmentUsername []string
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

// GetExcludeGroupRole return exclude roles
func (cd *ConfigData) GetExcludeGroupRole() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludeGroupRole
}

// RestrictDevelopmentUsername return exclude development username
func (cd *ConfigData) RestrictDevelopmentUsername() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.restrictDevelopmentUsername
}

// GetExcludeUsername return exclude username
func (cd *ConfigData) GetExcludeUsername() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludeUsername
}

// Interface to be used by consumer to check filters
type Interface interface {
	ToFilter(kind, namespace, name string) bool
	GetExcludeGroupRole() []string
	GetExcludeUsername() []string
	RestrictDevelopmentUsername() []string
}

// NewConfigData ...
func NewConfigData(rclient kubernetes.Interface, cmInformer informers.ConfigMapInformer, filterK8Resources, excludeGroupRole, excludeUsername string, log logr.Logger) *ConfigData {
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
	cd.restrictDevelopmentUsername = []string{"minikube-user", "kubernetes-admin"}

	//TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	if filterK8Resources != "" {
		cd.log.Info("init configuration from commandline arguments for filterK8Resources")
		cd.initFilters(filterK8Resources)
	}

	if excludeGroupRole != "" {
		cd.log.Info("init configuration from commandline arguments for excludeGroupRole")
		cd.initRbac("excludeRoles", excludeGroupRole)
	} else {
		cd.initRbac("excludeRoles", "")
	}

	if excludeUsername != "" {
		cd.log.Info("init configuration from commandline arguments for excludeUsername")
		cd.initRbac("excludeUsername", excludeUsername)
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
	// parse and load the configuration
	cd.mux.Lock()
	defer cd.mux.Unlock()
	// get resource filters
	filters, ok := cm.Data["resourceFilters"]
	if !ok {
		logger.V(4).Info("configuration: No resourceFilters defined in ConfigMap")
	} else {
		newFilters := parseKinds(filters)
		if reflect.DeepEqual(newFilters, cd.filters) {
			logger.V(4).Info("resourceFilters did not change")
		} else {
			logger.V(2).Info("Updated resource filters", "oldFilters", cd.filters, "newFilters", newFilters)
			// update filters
			cd.filters = newFilters
		}
	}

	// get resource filters
	excludeGroupRole, ok := cm.Data["excludeGroupRole"]
	if !ok {
		logger.V(4).Info("configuration: No excludeGroupRole defined in ConfigMap")
	}
	newExcludeGroupRoles := parseRbac(excludeGroupRole)
	newExcludeGroupRoles = append(newExcludeGroupRoles, defaultExcludeGroupRole...)
	if reflect.DeepEqual(newExcludeGroupRoles, cd.excludeGroupRole) {
		logger.V(4).Info("excludeGroupRole did not change")
	} else {
		logger.V(2).Info("Updated resource excludeGroupRoles", "oldExcludeGroupRole", cd.excludeGroupRole, "newExcludeGroupRole", newExcludeGroupRoles)
		// update filters
		cd.excludeGroupRole = newExcludeGroupRoles
	}

	// get resource filters
	excludeUsername, ok := cm.Data["excludeUsername"]
	if !ok {
		logger.V(4).Info("configuration: No excludeUsername defined in ConfigMap")
	} else {
		excludeUsernames := parseRbac(excludeUsername)
		if reflect.DeepEqual(excludeUsernames, cd.excludeUsername) {
			logger.V(4).Info("excludeGroupRole did not change")
		} else {
			logger.V(2).Info("Updated resource excludeUsernames", "oldExcludeUsername", cd.excludeUsername, "newExcludeUsername", excludeUsernames)
			// update filters
			cd.excludeUsername = excludeUsernames
		}
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

func (cd *ConfigData) initRbac(action, exclude string) {
	logger := cd.log
	// parse and load the configuration
	cd.mux.Lock()
	defer cd.mux.Unlock()
	rbac := parseRbac(exclude)
	logger.V(2).Info("Init resource ", action, exclude)
	// update filters
	if action == "excludeRoles" {
		cd.excludeGroupRole = rbac
		cd.excludeGroupRole = append(cd.excludeGroupRole, defaultExcludeGroupRole...)
	} else {
		cd.excludeUsername = rbac
	}

}

func (cd *ConfigData) unload(cm v1.ConfigMap) {
	logger := cd.log
	logger.Info("ConfigMap deleted, removing configuration filters", "name", cm.Name, "namespace", cm.Namespace)
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.filters = []k8Resource{}
	cd.excludeGroupRole = []string{}
	cd.excludeGroupRole = append(cd.excludeGroupRole, defaultExcludeGroupRole...)
	cd.excludeUsername = []string{}
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

func parseRbac(list string) []string {
	return strings.Split(list, ",")
}
