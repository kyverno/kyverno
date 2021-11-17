local_resource('compile-go', 'make tilt-compile', deps=['cmd/kyverno/main.go', 'go.mod', 'api', 'pkg', 'test'])

load('ext://restart_process', 'docker_build_with_restart')

docker_build_with_restart(
  'ghcr.io/kyverno/kyverno',
  './cmd/kyverno',
  entrypoint=['/kyverno'],
  dockerfile='./cmd/kyverno/localDockerfile',
  only=[
    'kyverno',
    'ca-certificates.crt'
  ],
  live_update=[
    sync('./cmd/kyverno/kyverno', '/kyverno'),
  ],
)

k8s_yaml(kustomize('config/tilt'))
k8s_resource(workload='kyverno', port_forwards=8000)
