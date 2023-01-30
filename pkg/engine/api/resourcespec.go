package api

// ResourceSpec resource action applied on
type ResourceSpec struct {
	Kind       string
	APIVersion string
	Namespace  string
	Name       string
	UID        string
}

// GetKey returns the key
func (rs ResourceSpec) GetKey() string {
	return rs.Kind + "/" + rs.Namespace + "/" + rs.Name
}
