package kubernetes

import "k8s.io/client-go/discovery"

func (c *clientset) Discovery() discovery.DiscoveryInterface {
	return c.inner.Discovery()
}
