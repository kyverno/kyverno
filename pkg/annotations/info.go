package annotations

type info struct {
	RKind string
	RNs   string
	RName string
	//TODO:Hack as slice makes the struct unhasable
	patch *[]byte
}

func newInfo(rkind, rns, rname string, patch *[]byte) info {
	return info{
		RKind: rkind,
		RNs:   rns,
		RName: rname,
		patch: patch,
	}
}
