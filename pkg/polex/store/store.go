package store

import (
	"sort"
	"sync"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
)

type Key struct {
	PolicyName string
	RuleName   string
}

type Store interface {
	Add(polex *kyvernov2beta1.PolicyException)
	Get(key Key) ([]*kyvernov2beta1.PolicyException, bool)
	Delete(polex *kyvernov2beta1.PolicyException)
}

type store struct {
	sync.RWMutex
	store map[string]*Trie
}

func New() Store {
	return &store{
		store: make(map[string]*Trie),
	}
}

func (l *store) Add(polex *kyvernov2beta1.PolicyException) {
	l.Lock()
	defer l.Unlock()

	for _, exception := range polex.Spec.Exceptions {
		trie, ok := l.store[exception.PolicyName]
		if !ok {
			trie = NewTrie()
			l.store[exception.PolicyName] = trie
		}

		for _, rule := range exception.RuleNames {
			trie.Insert(rule, polex)
		}
	}
}

func (l *store) Get(key Key) ([]*kyvernov2beta1.PolicyException, bool) {
	l.RLock()
	defer l.RUnlock()

	entry, ok := l.store[key.PolicyName]
	if !ok {
		return nil, false
	}

	polexes := entry.Search(key.RuleName)

	sort.Slice(polexes, func(i, j int) bool {
		return polexes[i].Name < polexes[j].Name
	})

	return polexes, true
}

func (l *store) Delete(polex *kyvernov2beta1.PolicyException) {
	l.Lock()
	defer l.Unlock()

	for _, exception := range polex.Spec.Exceptions {
		trie, ok := l.store[exception.PolicyName]
		if !ok {
			continue
		}

		for _, rule := range exception.RuleNames {
			trie.Delete(rule, polex.UID)
		}
	}
}
