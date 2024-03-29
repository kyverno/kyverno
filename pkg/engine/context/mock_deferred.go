package context

import "fmt"

type mockLoader struct {
	name         string
	level        int
	value        interface{}
	query        string
	hasLoaded    bool
	invocations  int
	eventHandler func(event string)
	ctx          Interface
}

func (ml *mockLoader) Name() string {
	return ml.name
}

func (ml *mockLoader) SetLevel(level int) {
	ml.level = level
}

func (ml *mockLoader) GetLevel() int {
	return ml.level
}

func (ml *mockLoader) HasLoaded() bool {
	return ml.hasLoaded
}

func (ml *mockLoader) LoadData() error {
	ml.invocations++
	err := ml.ctx.AddVariable(ml.name, ml.value)
	if err != nil {
		return err
	}

	// simulate a JMESPath evaluation after loading
	if err := ml.executeQuery(); err != nil {
		return err
	}

	ml.hasLoaded = true
	if ml.eventHandler != nil {
		event := fmt.Sprintf("%s=%v", ml.name, ml.value)
		ml.eventHandler(event)
	}

	return nil
}

func (ml *mockLoader) executeQuery() error {
	if ml.query == "" {
		return nil
	}

	results, err := ml.ctx.Query(ml.query)
	if err != nil {
		return err
	}

	return ml.ctx.AddVariable(ml.name, results)
}

func (ml *mockLoader) setEventHandler(eventHandler func(string)) {
	ml.eventHandler = eventHandler
}

func AddMockDeferredLoader(ctx Interface, name string, value interface{}) (*mockLoader, error) {
	return addDeferredWithQuery(ctx, name, value, "")
}

func addDeferredWithQuery(ctx Interface, name string, value interface{}, query string) (*mockLoader, error) {
	loader := &mockLoader{
		name:  name,
		value: value,
		ctx:   ctx,
		query: query,
	}

	d, err := NewDeferredLoader(name, loader, logger)
	if err != nil {
		return loader, err
	}

	err = ctx.AddDeferredLoader(d)
	if err != nil {
		return nil, err
	}
	return loader, nil
}
