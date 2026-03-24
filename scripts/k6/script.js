import { Kubernetes } from 'k6/x/kubernetes';

const podSpec = {
    apiVersion: "v1",
    kind: "Pod",
    metadata: {
        generateName: "busybox",
        namespace: "testns"
    },
    spec: {
        containers: [
            {
                name: "busybox",
                image: "busybox",
                command: ["sh", "-c", "sleep 30"]
            }
        ],
        affinity: {
            nodeAffinity: {
                requiredDuringSchedulingIgnoredDuringExecution: {
                    nodeSelectorTerms: [
                        {
                            matchExpressions: [
                                {
                                    key: "type",
                                    operator: "In",
                                    values: ["kwok"],
                                }
                            ]
                        }
                    ]
                }
            }
        },
        tolerations: [
            {
                key: "kwok.x-k8s.io/node",
                operator: "Exists",
                effect: "NoSchedule",
            }
        ]
    }
}

export default function () {
    const kubernetes = new Kubernetes();

    const pod = kubernetes.create(podSpec)

    // const helpers = kubernetes.helpers()

    // const timeout = 10
    // if (!helpers.waitPodRunning(pod.metadata.name, timeout)) {
    //     console.log(`"pod ${pod.metadata.name} not ready after ${timeout} seconds`)
    // }

    // kubernetes.delete("Pod", pod.metadata.name, pod.metadata.namespace)
}
