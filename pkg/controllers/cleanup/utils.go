package cleanup

import (
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getCronJobForTriggerResource(pol kyvernov1alpha1.CleanupPolicyInterface) *batchv1.CronJob {
	namespace := pol.GetNamespace()
	if namespace == "" {
		namespace = "kyverno"
	}
	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(pol.GetUID()),
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: pol.GetAPIVersion(),
					Kind:       pol.GetKind(),
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
									Image: "bitnami/kubectl:latest",
									Args: []string{
										"/bin/sh",
										"-c",
										`echo "Hello World"`,
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
