package policyreport

import cmap "github.com/orcaman/concurrent-map"

type concurrentMap struct{ cmap.ConcurrentMap }

func (m concurrentMap) increase(ns string) {
	count, ok := m.Get(ns)
	if ok && count != -1 {
		m.Set(ns, count.(int)+1)
	} else {
		m.Set(ns, 1)
	}
}

func (m concurrentMap) decrease(keyHash string) {
	_, ns := parseKeyHash(keyHash)
	count, ok := m.Get(ns)
	if ok && count.(int) > 0 {
		m.Set(ns, count.(int)-1)
	} else {
		m.Set(ns, 0)
	}
}
