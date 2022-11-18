package cleanup

import (
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) getPolicy(namespace, name string) (kyvernov1alpha1.CleanupPolicyInterface, error) {
	if namespace == "" {
		cpolicy, err := c.cpolLister.Get(name)
		if err != nil {
			return nil, err
		}
		return cpolicy, nil
	} else {
		policy, err := c.polLister.CleanupPolicies(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return policy, nil
	}
}

func getCronJobForTriggerResource(pol kyvernov1alpha1.CleanupPolicyInterface) *batchv1.CronJob {
	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cleanupcj",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
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
