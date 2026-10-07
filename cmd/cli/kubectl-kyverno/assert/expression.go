package assert

import (
	"context"
	"reflect"
	"regexp"
)

var (
	foreachRegex    = regexp.MustCompile(`^~(\w+)?\.(.*)`)
	bindingRegex    = regexp.MustCompile(`(.*?)\s*->\s*(\w+)$`)
	escapeRegex     = regexp.MustCompile(`^\\(.+)\\$`)
	expressionRegex = regexp.MustCompile(`^\((.+)\)$`)
)

type expression struct {
	foreach     bool
	foreachName string
	statement   string
	binding     string
	expression  bool
}

func parseExpressionRegex(_ context.Context, in string) *expression {
	expression := &expression{}
	// 1. match foreach
	if match := foreachRegex.FindStringSubmatch(in); match != nil {
		expression.foreach = true
		expression.foreachName = match[1]
		in = match[2]
	}
	// 2. match binding
	if match := bindingRegex.FindStringSubmatch(in); match != nil {
		expression.binding = match[2]
		in = match[1]
	}
	// 3. match escape, if there's no escaping then match expression
	if match := escapeRegex.FindStringSubmatch(in); match != nil {
		in = match[1]
	} else if match := expressionRegex.FindStringSubmatch(in); match != nil {
		expression.expression = true
		in = match[1]
	}
	// parse statement
	expression.statement = in
	if expression.statement == "" {
		return nil
	}
	return expression
}

func parseExpression(ctx context.Context, value any) *expression {
	if getKind(value) != reflect.String {
		return nil
	}
	return parseExpressionRegex(ctx, reflect.ValueOf(value).String())
}
