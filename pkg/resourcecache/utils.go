package resourcecache

func (resc *ResourceCache) matchGVRKey(key string) bool {
	if len(resc.match) == 0 {
		return true
	}
	ok := false
	for _, mkey := range resc.match {
		if key == mkey {
			ok = true
			break
		}
	}
	return ok
}

func (resc *ResourceCache) excludeGVRKey(key string) bool {
	if len(resc.exclude) == 0 {
		return false
	}
	ok := true
	for _, ekey := range resc.exclude {
		if key == ekey {
			ok = false
			break
		}
	}
	return ok
}