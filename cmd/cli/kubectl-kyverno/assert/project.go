package assert

import (
	"context"
	"errors"
	"reflect"
)

type projection struct {
	foreach     bool
	foreachName string
	binding     string
	result      any
}

func project(ctx context.Context, key any, value any, bindings Bindings) (*projection, error) {
	expression := parseExpression(ctx, key)
	if expression != nil {
		if expression.expression {
			projected, err := execute(expression.statement, value, bindings)
			if err != nil {
				return nil, err
			}
			return &projection{
				foreach:     expression.foreach,
				foreachName: expression.foreachName,
				binding:     expression.binding,
				result:      projected,
			}, nil
		} else {
			if value == nil {
				return nil, nil
			} else if getKind(value) == reflect.Map {
				mapValue := reflect.ValueOf(value).MapIndex(reflect.ValueOf(expression.statement))
				if !mapValue.IsValid() {
					return nil, nil
				}
				return &projection{
					foreach:     expression.foreach,
					foreachName: expression.foreachName,
					binding:     expression.binding,
					result:      mapValue.Interface(),
				}, nil
			}
		}
	}
	if getKind(value) == reflect.Map {
		mapValue := reflect.ValueOf(value).MapIndex(reflect.ValueOf(key))
		if !mapValue.IsValid() {
			return nil, nil
		}
		return &projection{
			result: mapValue.Interface(),
		}, nil
	}
	return nil, errors.New("projection not recognized")
}
