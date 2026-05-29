import { Kubernetes } from 'k6/x/kubernetes';
import { check } from 'k6';

// Pod spec targeting KWOK fake nodes, deliberately valid against any
// ValidatingPolicy test policy in test/load/policies/vpol-*.yaml.
const podSpec = {
    apiVersion: "v1",
    kind: "Pod",
    metadata: {
        generateName: "perf-vpol-",
        namespace: "testns",
        labels: {
            "app": "perf-test",
            "env": "prod",
            "team": "platform",
        },
        annotations: {
            "owner": "perf-test",
        },
    },
    spec: {
        securityContext: {
            runAsNonRoot: true,
            runAsUser: 1000,
            seccompProfile: { type: "RuntimeDefault" },
        },
        containers: [
            {
                name: "busybox",
                image: "busybox:latest",
                command: ["sh", "-c", "sleep 30"],
                resources: {
                    requests: { cpu: "1m", memory: "8Mi" },
                    limits:   { memory: "32Mi" },
                },
                securityContext: {
                    allowPrivilegeEscalation: false,
                    capabilities: { drop: ["ALL"] },
                    readOnlyRootFilesystem: true,
                    runAsNonRoot: true,
                },
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
