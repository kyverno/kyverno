package userinfo

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	rbaclister "k8s.io/client-go/listers/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	clusterroleKind = "ClusterRole"
	roleKind        = "Role"
	// saPrefix represents service account prefix in admission requests
	saPrefix = "system:serviceaccount:"
)

type lister struct {
	clientset            kubernetes.Interface
	filter               string
	gkeParsedProjectName string
	subjectKind          string
	rbacSubjectsByScope  map[string]rbacSubject
}

type rbacSubject struct {
	Kind         string
	RolesByScope map[string][]simpleRole
}

type simpleRole struct {
	Kind   string
	Name   string
	Source simpleRoleSource
}

type simpleRoleSource struct {
	Kind string
	Name string
}

func (rbacSubj *rbacSubject) addRoleBinding(roleBinding *rbacv1.RoleBinding) {
	simpleRole := simpleRole{
		Name: roleBinding.RoleRef.Name,
		Source: simpleRoleSource{
			Name: roleBinding.Name,
			Kind: "RoleBinding",
		},
	}

	simpleRole.Kind = roleBinding.RoleRef.Kind
	rbacSubj.RolesByScope[roleBinding.Namespace] = append(rbacSubj.RolesByScope[roleBinding.Namespace], simpleRole)
}

func (rbacSubj *rbacSubject) addClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding) {
	simpleRole := simpleRole{
		Name:   clusterRoleBinding.RoleRef.Name,
		Source: simpleRoleSource{Name: clusterRoleBinding.Name, Kind: "ClusterRoleBinding"},
	}

	simpleRole.Kind = clusterRoleBinding.RoleRef.Kind
	scope := "cluster-wide"
	rbacSubj.RolesByScope[scope] = append(rbacSubj.RolesByScope[scope], simpleRole)
}

func PrintRbacBindings(l *lister, outputFormat string) (roles []string, clusterRoles []string) {
	if len(l.rbacSubjectsByScope) < 1 {
		fmt.Println("No RBAC Bindings found")
		return
	}

	names := make([]string, 0, len(l.rbacSubjectsByScope))
	for name := range l.rbacSubjectsByScope {
		names = append(names, name)
	}
	sort.Strings(names)

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, ' ', 0)

	if outputFormat == "wide" {
		fmt.Fprintln(w, "SUBJECT\t SCOPE\t ROLE\t SOURCE")
	} else {
		fmt.Fprintln(w, "SUBJECT\t SCOPE\t ROLE")
	}

	for _, subjectName := range names {
		rbacSubject := l.rbacSubjectsByScope[subjectName]
		for scope, simpleRoles := range rbacSubject.RolesByScope {
			for _, simpleRole := range simpleRoles {
				if outputFormat == "wide" {
					fmt.Fprintf(w, "%s/%s \t %s\t %s/%s\t %s/%s\n", rbacSubject.Kind, subjectName, scope, simpleRole.Kind, simpleRole.Name, simpleRole.Source.Kind, simpleRole.Source.Name)
				} else {
					fmt.Fprintf(w, "%s \t %s\t %s/%s\n", subjectName, scope, simpleRole.Kind, simpleRole.Name)
					switch simpleRole.Kind {
					case roleKind:
						roles = append(roles, simpleRole.Name)
					case clusterroleKind:
						clusterRoles = append(clusterRoles, simpleRole.Name)
					}
				}
			}
		}
	}
	w.Flush()
	return roles, clusterRoles
}

func LoadRoleBindings(l *lister) error {
	roleBindings, err := l.clientset.RbacV1().RoleBindings("").List(context.Background(), metav1.ListOptions{})

	if err != nil {
		fmt.Println("Error loading role bindings")
		return err
	}

	for _, roleBinding := range roleBindings.Items {
		for _, subject := range roleBinding.Subjects {
			if l.nameMatches(subject.Name) && l.kindMatches(subject.Kind) {
				subjectKey := subject.Name
				if subject.Kind == "ServiceAccount" {
					subjectKey = fmt.Sprintf("%s:%s", subject.Namespace, subject.Name)
				}
				if rbacSubj, exist := l.rbacSubjectsByScope[subjectKey]; exist {
					rbacSubj.addRoleBinding(&roleBinding)
				} else {
					rbacSubj := rbacSubject{
						Kind:         subject.Kind,
						RolesByScope: make(map[string][]simpleRole),
					}
					rbacSubj.addRoleBinding(&roleBinding)

					l.rbacSubjectsByScope[subjectKey] = rbacSubj
				}
			}
		}
	}

	return nil
}

func (l *lister) nameMatches(name string) bool {
	return l.filter == "" || strings.Contains(name, l.filter)
}

func (l *lister) kindMatches(kind string) bool {
	if l.subjectKind == "" {
		return true
	}

	lowerKind := strings.ToLower(kind)

	return lowerKind == l.subjectKind
}

func (l *lister) LoadClusterRoleBindings() error {
	clusterRoleBindings, err := l.clientset.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})

	if err != nil {
		fmt.Println("Error loading cluster role bindings")
		return err
	}

	for _, clusterRoleBinding := range clusterRoleBindings.Items {
		for _, subject := range clusterRoleBinding.Subjects {
			if l.nameMatches(subject.Name) && l.kindMatches(subject.Kind) {
				subjectKey := subject.Name
				if subject.Kind == "ServiceAccount" {
					subjectKey = fmt.Sprintf("%s:%s", subject.Namespace, subject.Name)
				}
				if rbacSubj, exist := l.rbacSubjectsByScope[subjectKey]; exist {
					rbacSubj.addClusterRoleBinding(&clusterRoleBinding)
				} else {
					rbacSubj := rbacSubject{
						Kind:         subject.Kind,
						RolesByScope: make(map[string][]simpleRole),
					}
					rbacSubj.addClusterRoleBinding(&clusterRoleBinding)

					l.rbacSubjectsByScope[subjectKey] = rbacSubj
				}
			}
		}
	}

	return nil
}

//GetRoleRef gets the list of roles and cluster roles for the incoming api-request
func GetRoleRef(rbLister rbaclister.RoleBindingLister, crbLister rbaclister.ClusterRoleBindingLister, request *admissionv1.AdmissionRequest, dynamicConfig config.Interface) ([]string, []string, error) {
	keys := append(request.UserInfo.Groups, request.UserInfo.Username)
	if utils.SliceContains(keys, dynamicConfig.GetExcludeGroupRole()...) {
		return nil, nil, nil
	}
	// rolebindings
	roleBindings, err := rbLister.List(labels.Everything())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list rolebindings: %v", err)
	}
	rs, crs := getRoleRefByRoleBindings(roleBindings, request.UserInfo)
	// clusterrolebindings
	clusterroleBindings, err := crbLister.List(labels.NewSelector())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list clusterrolebindings: %v", err)
	}
	crs = append(crs, getRoleRefByClusterRoleBindings(clusterroleBindings, request.UserInfo)...)
	return rs, crs, nil
}

func getRoleRefByRoleBindings(roleBindings []*rbacv1.RoleBinding, userInfo authenticationv1.UserInfo) (roles []string, clusterRoles []string) {
	for _, rolebinding := range roleBindings {
		for _, subject := range rolebinding.Subjects {
			if matchSubjectsMap(subject, userInfo) {
				switch rolebinding.RoleRef.Kind {
				case roleKind:
					roles = append(roles, rolebinding.Namespace+":"+rolebinding.RoleRef.Name)
				case clusterroleKind:
					clusterRoles = append(clusterRoles, rolebinding.RoleRef.Name)
				}
			}
		}
	}
	return roles, clusterRoles
}

// RoleRef in ClusterRoleBindings can only reference a ClusterRole in the global namespace
func getRoleRefByClusterRoleBindings(clusterroleBindings []*rbacv1.ClusterRoleBinding, userInfo authenticationv1.UserInfo) (clusterRoles []string) {
	for _, clusterRoleBinding := range clusterroleBindings {
		for _, subject := range clusterRoleBinding.Subjects {
			if matchSubjectsMap(subject, userInfo) {
				if clusterRoleBinding.RoleRef.Kind == clusterroleKind {
					clusterRoles = append(clusterRoles, clusterRoleBinding.RoleRef.Name)
				}
			}
		}
	}
	return clusterRoles
}

// matchSubjectsMap checks if userInfo found in subject
// return true directly if found a match
// subject.kind can only be ServiceAccount, User and Group
func matchSubjectsMap(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	if strings.Contains(userInfo.Username, saPrefix) {
		return matchServiceAccount(subject, userInfo)
	}
	return matchUserOrGroup(subject, userInfo)
}

// matchServiceAccount checks if userInfo sa matche the subject sa
// serviceaccount represents as saPrefix:namespace:name in userInfo
func matchServiceAccount(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	subjectServiceAccount := subject.Namespace + ":" + subject.Name
	if userInfo.Username[len(saPrefix):] != subjectServiceAccount {
		return false
	}
	log.Log.V(3).Info(fmt.Sprintf("found a matched service account not match: %s", subjectServiceAccount))
	return true
}

// matchUserOrGroup checks if userInfo contains user or group info in a subject
func matchUserOrGroup(subject rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	keys := append(userInfo.Groups, userInfo.Username)
	for _, key := range keys {
		if subject.Name == key {
			log.Log.V(3).Info(fmt.Sprintf("found a matched user/group '%v' in request userInfo: %v", subject.Name, keys))
			return true
		}
	}
	return false
}
