import { Kubernetes } from 'k6/x/kubernetes';
import { check } from 'k6';

// Pod spec targeting KWOK fake nodes. Labels and annotations are intentionally
// minimal so MutatingPolicy test policies can add/overwrite them and the diff
// is visible in Prometheus mutation metrics.
const podSpec = {
    apiVersion: "v1",
    kind: "Pod",
    metadata: {
        generateName: "perf-mpol-",
        namespace: "testns",
    },
    spec: {
        containers: [
            {
                name: "busybox",
                image: "busybox:latest",
                command: ["sh", "-c", "sleep 30"],
            }
        ],
        affinity: {
            nodeAffinity: {
                requiredDuringSchedulingIgnoredDuringExecution: {
                    nodeSelectorTerms: [
                        {
                            matchExpressions: [
                                { key: "type", operator: "In", values: ["kwok"] }
                            ]
                        }
                    ]
                }
            }
        },
        tolerations: [
            { key: "kwok.x-k8s.io/node", operator: "Exists", effect: "NoSchedule" }
        ],
    }
};

export default function () {
    const kubernetes = new Kubernetes();
    let err = null;
    try {
        kubernetes.create(podSpec);
    } catch (e) {
        err = e;
    }
    check(err, { 'pod created successfully': (e) => e === null });
}
