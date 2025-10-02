package webhook

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/kyverno/kyverno/pkg/cel/compiler"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type compiledExpression struct {
	expression string
	hash       string
	errors     field.ErrorList
	isValid    bool
	compiledAt time.Time
	isStored   bool
}

type expressionCache struct {
	mu                     sync.RWMutex
	cache                  map[string]*compiledExpression
	preexistingExpressions map[string]bool
}

func NewExpressionCache() *expressionCache {
	return &expressionCache{
		cache:                  make(map[string]*compiledExpression),
		preexistingExpressions: make(map[string]bool),
	}
}

func (c *expressionCache) GetOrCompile(condition admissionregistration.MatchCondition) *compiledExpression {
	hash := c.hashMatchCondition(condition)

	c.mu.RLock()
	if cached, exists := c.cache[hash]; exists {
		c.mu.RUnlock()
		return cached
	}
	c.mu.RUnlock()

	c.mu.RLock()
	isPreexisting := c.preexistingExpressions[condition.Expression]
	c.mu.RUnlock()

	errors := compiler.CompileMatchConditionsWithKubernetesEnv([]admissionregistration.MatchCondition{condition}, c.preexistingExpressions)

	compiled := &compiledExpression{
		expression: condition.Expression,
		hash:       hash,
		errors:     errors,
		isValid:    len(errors) == 0,
		compiledAt: time.Now(),
		isStored:   isPreexisting,
	}

	c.mu.Lock()
	c.cache[hash] = compiled
	c.preexistingExpressions[condition.Expression] = true
	c.mu.Unlock()

	return compiled
}

func (c *expressionCache) FilterValidMatchConditions(conditions []admissionregistration.MatchCondition) []admissionregistration.MatchCondition {
	var validConditions []admissionregistration.MatchCondition

	for _, condition := range conditions {
		compiled := c.GetOrCompile(condition)
		if compiled.isValid {
			validConditions = append(validConditions, condition)
		}
	}

	return validConditions
}

func (c *expressionCache) ValidateMatchConditions(conditions []admissionregistration.MatchCondition) ([]admissionregistration.MatchCondition, field.ErrorList) {
	var validConditions []admissionregistration.MatchCondition
	var allErrors field.ErrorList

	for i, condition := range conditions {
		compiled := c.GetOrCompile(condition)
		if compiled.isValid {
			validConditions = append(validConditions, condition)
		} else {
			for _, err := range compiled.errors {
				allErrors = append(allErrors, field.Invalid(
					field.NewPath("matchConditions").Index(i).Child("expression"),
					condition.Expression,
					err.Detail,
				))
			}
		}
	}

	return validConditions, allErrors
}

func (c *expressionCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*compiledExpression)
	c.preexistingExpressions = make(map[string]bool)
}

func (c *expressionCache) InvalidateOnPolicyChange() {
	c.Invalidate()
}

// AddExpression adds a new expression to the cache and preexisting set
func (c *expressionCache) AddExpression(condition admissionregistration.MatchCondition) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add to preexisting expressions for future compilations
	c.preexistingExpressions[condition.Expression] = true

	// Pre-compile and cache the expression
	hash := c.hashMatchCondition(condition)
	errors := compiler.CompileMatchConditionsWithKubernetesEnv([]admissionregistration.MatchCondition{condition}, c.preexistingExpressions)

	compiled := &compiledExpression{
		expression: condition.Expression,
		hash:       hash,
		errors:     errors,
		isValid:    len(errors) == 0,
		compiledAt: time.Now(),
		isStored:   true,
	}

	c.cache[hash] = compiled
}

func (c *expressionCache) RemoveExpression(condition admissionregistration.MatchCondition) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hash := c.hashMatchCondition(condition)
	delete(c.cache, hash)
}

func (c *expressionCache) AddPolicyExpressions(conditions []admissionregistration.MatchCondition) {
	for _, condition := range conditions {
		c.AddExpression(condition)
	}
}

func (c *expressionCache) RemovePolicyExpressions(conditions []admissionregistration.MatchCondition) {
	for _, condition := range conditions {
		c.RemoveExpression(condition)
	}
}

func (c *expressionCache) hashMatchCondition(condition admissionregistration.MatchCondition) string {
	content := fmt.Sprintf("%s:%s", condition.Name, condition.Expression)
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}
