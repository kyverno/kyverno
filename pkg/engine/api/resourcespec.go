package api

// ResourceSpec resource action applied on
type ResourceSpec struct {
	Kind       string
	APIVersion string
	Namespace  string
	Name       string
	UID        string
}

// String implements Stringer interface
func (rs ResourceSpec) String() string {
	return rs.Kind + "/" + rs.Namespace + "/" + rs.Name
}
