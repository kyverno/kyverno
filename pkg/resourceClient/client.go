package resourceClient

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

func GetResouce(clientSet *kubernetes.Clientset, kind string, resourceNamespace string, resourceName string) (runtime.Object, error) {
	switch kind {
	case "Deployment":
		{
			obj, err := clientSet.AppsV1().Deployments(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "Pods":
		{
			obj, err := clientSet.CoreV1().Pods(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "ConfigMap":
		{
			obj, err := clientSet.CoreV1().ConfigMaps(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "CronJob":
		{
			obj, err := clientSet.BatchV1beta1().CronJobs(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "Endpoints":
		{
			obj, err := clientSet.CoreV1().Endpoints(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "HorizontalPodAutoscaler":
		{
			obj, err := clientSet.AutoscalingV1().HorizontalPodAutoscalers(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "Ingress":
		{
			obj, err := clientSet.ExtensionsV1beta1().Ingresses(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "Job":
		{
			obj, err := clientSet.BatchV1().Jobs(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "LimitRange":
		{
			obj, err := clientSet.CoreV1().LimitRanges(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "Namespace":
		{
			obj, err := clientSet.CoreV1().Namespaces().Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "NetworkPolicy":
		{
			obj, err := clientSet.NetworkingV1().NetworkPolicies(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "PersistentVolumeClaim":
		{
			obj, err := clientSet.CoreV1().PersistentVolumeClaims(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "PodDisruptionBudget":
		{
			obj, err := clientSet.PolicyV1beta1().PodDisruptionBudgets(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "PodTemplate":
		{
			obj, err := clientSet.CoreV1().PodTemplates(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "ResourceQuota":
		{
			obj, err := clientSet.CoreV1().ResourceQuotas(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "Secret":
		{
			obj, err := clientSet.CoreV1().Secrets(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "Service":
		{
			obj, err := clientSet.CoreV1().Services(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}
	case "StatefulSet":
		{
			obj, err := clientSet.AppsV1().StatefulSets(resourceNamespace).Get(resourceName, meta_v1.GetOptions{})
			if err != nil {
				return nil, err
			}
			return obj, nil
		}

	default:
		return nil, nil
	}
}
