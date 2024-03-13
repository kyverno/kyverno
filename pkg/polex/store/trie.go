package store

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"k8s.io/apimachinery/pkg/types"
)

type Node struct {
	children []*Node
	polexes  []*kyvernov2beta1.PolicyException
}

type Trie struct {
	root *Node
}

func NewTrie() *Trie {
	return &Trie{
		root: &Node{children: make([]*Node, 128)},
	}
}

func (t *Trie) Insert(key string, polex *kyvernov2beta1.PolicyException) {
	currentNode := t.root
	for i := range key {
		charIndex := int(key[i])
		if currentNode.children[charIndex] == nil {
			currentNode.children[charIndex] = &Node{children: make([]*Node, 128), polexes: make([]*kyvernov2beta1.PolicyException, 0, 8)}
		}
		currentNode = currentNode.children[charIndex]
	}
	currentNode.polexes = append(currentNode.polexes, polex)
}

func (t *Trie) Search(key string) []*kyvernov2beta1.PolicyException {
	results := make([]*kyvernov2beta1.PolicyException, 0, 128)
	uniquePolexes := make(map[types.UID]bool, 128)

	t.searchHelper(t.root, key, 0, &results, &uniquePolexes)

	return results
}

func (t *Trie) searchHelper(node *Node, key string, index int, results *[]*kyvernov2beta1.PolicyException, uniquePolexes *map[types.UID]bool) {
	if node == nil {
		return
	}

	if index == len(key) {
		for i := 0; i < len(node.polexes); i++ {
			if !(*uniquePolexes)[node.polexes[i].UID] {
				*results = append(*results, node.polexes[i])
				(*uniquePolexes)[node.polexes[i].UID] = true
			}
		}
		return
	}

	char := rune(key[index])
	charIndex := int(char)

	if child := node.children[charIndex]; child != nil {
		t.searchHelper(child, key, index+1, results, uniquePolexes)
	}

	if child := node.children['*']; child != nil {
		t.searchHelper(child, key, index, results, uniquePolexes)
		t.searchHelper(node, key, index+1, results, uniquePolexes)
		t.searchHelper(child, key, index+1, results, uniquePolexes)
	}

	// if child := node.children['?']; child != nil {
	// 	t.searchHelper(child, key, index+1, results, uniquePolexes)
	// }
}

func (t *Trie) Delete(key string, uid types.UID) {
	currentNode := t.root
	for i := range key {
		charIndex := int(key[i])
		if currentNode.children[charIndex] == nil {
			return
		}
		currentNode = currentNode.children[charIndex]
	}

	for i := range currentNode.polexes {
		if currentNode.polexes[i].UID == uid {
			currentNode.polexes = append(currentNode.polexes[:i], currentNode.polexes[i+1:]...)
		}
	}
}
