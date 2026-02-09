package compiler

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

var (
	VariablesType     = types.NewObjectType("kyverno.variables")
	variablesTypeType = types.NewTypeTypeWithParam(VariablesType)
)

type VariablesProvider struct {
	inner  types.Provider
	fields map[string]*types.Type
	names  []string
}

func NewVariablesProvider(inner types.Provider) *VariablesProvider {
	return &VariablesProvider{
		inner:  inner,
		fields: make(map[string]*types.Type),
	}
}

func (p *VariablesProvider) RegisterField(name string, t *types.Type) {
	p.fields[name] = t
	p.names = append(p.names, name)
}

func (p *VariablesProvider) EnumValue(enumName string) ref.Val {
	return p.inner.EnumValue(enumName)
}

func (p *VariablesProvider) FindIdent(identName string) (ref.Val, bool) {
	return p.inner.FindIdent(identName)
}

func (p *VariablesProvider) FindStructType(structType string) (*types.Type, bool) {
	if structType == VariablesType.DeclaredTypeName() {
		return variablesTypeType, true
	}
	return p.inner.FindStructType(structType)
}

func (p *VariablesProvider) FindStructFieldNames(structType string) ([]string, bool) {
	if structType == VariablesType.DeclaredTypeName() {
		return p.names, true
	}
	return p.inner.FindStructFieldNames(structType)
}

func (p *VariablesProvider) FindStructFieldType(structType, fieldName string) (*types.FieldType, bool) {
	if structType == VariablesType.DeclaredTypeName() {
		if t, ok := p.fields[fieldName]; ok {
			return &types.FieldType{
				Type: t,
			}, true
		}
		return nil, false
	}
	return p.inner.FindStructFieldType(structType, fieldName)
}

func (p *VariablesProvider) NewValue(structType string, fields map[string]ref.Val) ref.Val {
	return p.inner.NewValue(structType, fields)
}
