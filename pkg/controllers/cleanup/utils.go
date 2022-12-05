package cleanup

import (
	"fmt"

	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func getCronJobForTriggerResource(pol kyvernov1alpha1.CleanupPolicyInterface) *batchv1.CronJob {
	// TODO: find a better way to do that, it looks like resources returned by WATCH don't have the GVK
	apiVersion := "kyverno.io/v1alpha1"
	kind := "CleanupPolicy"
	if pol.GetNamespace() == "" {
		kind = "ClusterCleanupPolicy"
	}
	// TODO: error
	policyName, _ := cache.MetaNamespaceKeyFunc(pol)
	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(pol.GetUID()),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: apiVersion,
					Kind:       kind,
					Name:       pol.GetName(),
					UID:        pol.GetUID(),
				},
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: pol.GetSpec().Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:  "cleanup",
									Image: "curlimages/curl:7.86.0",
									Args: []string{
										"-k",
										// TODO: ca
										// "--cacert",
										// "/tmp/ca.crt",
										// TODO: this should be configurable
										fmt.Sprintf("https://cleanup-controller.kyverno.svc/cleanup?policy=%s", policyName),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return cronjob
}
