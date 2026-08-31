package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v4"
	"k8s.io/client-go/kubernetes"
)

type WebhookAuthenticator struct {
	inner      http.Handler
	kubeClient kubernetes.Interface
	enabled    bool
	jwks       *jose.JSONWebKeySet
	jwksMutex  sync.RWMutex
	lastFetch  time.Time
}

func NewWebhookAuthenticator(inner http.Handler, client kubernetes.Interface, enabled bool) http.Handler {
	if !enabled {
		return inner
	}
	return &WebhookAuthenticator{
		inner:      inner,
		kubeClient: client,
		enabled:    enabled,
	}
}

func (a *WebhookAuthenticator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !a.enabled {
		a.inner.ServeHTTP(w, r)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized: Missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	if err := a.verifyToken(r.Context(), tokenString, r); err != nil {
		http.Error(w, fmt.Sprintf("Unauthorized: %v", err), http.StatusUnauthorized)
		return
	}

	a.inner.ServeHTTP(w, r)
}

func (a *WebhookAuthenticator) fetchJWKS(ctx context.Context) (*jose.JSONWebKeySet, error) {
	a.jwksMutex.RLock()
	if a.jwks != nil && time.Since(a.lastFetch) < time.Hour {
		jwks := a.jwks
		a.jwksMutex.RUnlock()
		return jwks, nil
	}
	a.jwksMutex.RUnlock()

	a.jwksMutex.Lock()
	defer a.jwksMutex.Unlock()

	// Double check inside lock
	if a.jwks != nil && time.Since(a.lastFetch) < time.Hour {
		return a.jwks, nil
	}

	restClient := a.kubeClient.Discovery().RESTClient()
	if restClient == nil {
		return nil, fmt.Errorf("RESTClient is nil, cannot fetch JWKS")
	}

	res := restClient.Get().AbsPath("/openid/v1/jwks").Do(ctx)
	if res.Error() != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", res.Error())
	}

	raw, err := res.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to read JWKS response: %w", err)
	}

	var jwks jose.JSONWebKeySet
	if err := json.Unmarshal(raw, &jwks); err != nil {
		return nil, fmt.Errorf("failed to parse JWKS: %w", err)
	}

	a.jwks = &jwks
	a.lastFetch = time.Now()
	return a.jwks, nil
}

func (a *WebhookAuthenticator) verifyToken(ctx context.Context, tokenString string, r *http.Request) error {
	jwks, err := a.fetchJWKS(ctx)
	if err != nil {
		return err
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}))
	token, err := parser.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid in token header")
		}

		keys := jwks.Key(kid)
		if len(keys) == 0 {
			return nil, fmt.Errorf("key %s not found in JWKS", kid)
		}

		rsaKey, ok := keys[0].Key.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("key is not RSA public key")
		}
		return rsaKey, nil
	})
	if err != nil {
		return fmt.Errorf("invalid token signature: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return errors.New("invalid token claims")
	}

	if !verifyAudience(claims, r.URL.Path) {
		return errors.New("invalid token audience")
	}

	allowedGroups, ok := claims["webhook-authentication.k8s.io/allowedAPIGroup"].([]interface{})
	if !ok {
		return errors.New("missing or invalid allowedAPIGroup claim")
	}

	hasAccess := false
	for _, g := range allowedGroups {
		groupStr, ok := g.(string)
		if !ok {
			continue
		}
		if groupStr == "*" || groupStr == "kyverno.io" {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return errors.New("token not authorized for required APIGroup")
	}

	return nil
}

func verifyAudience(claims jwt.MapClaims, reqPath string) bool {
	audiences := claims["aud"]
	if audiences == nil {
		return false
	}

	switch aud := audiences.(type) {
	case string:
		return strings.HasSuffix(aud, reqPath)
	case []interface{}:
		for _, a := range aud {
			if aStr, ok := a.(string); ok {
				if strings.HasSuffix(aStr, reqPath) {
					return true
				}
			}
		}
	}
	return false
}
