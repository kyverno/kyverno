package store

var Mock bool
var ContextVar Context

func SetMock(mock bool) {
	Mock = mock
}

func GetMock() bool {
	return Mock
}

func SetContext(context Context) {
	ContextVar = context
}

func GetContext() Context {
	return ContextVar
}

type Context struct {
	Policies []Policy `json:"policies"`
}

type Policy struct {
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Vars map[string]string `json:"vars"`
}
