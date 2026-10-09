// Package provenance authenticates controller-created legacy generate resources.
package provenance

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// SecretName is the installation-scoped, persistent signing key.
	SecretName = "kyverno-generate-provenance"
	// Annotation stores the authenticated provenance of a generated resource.
	Annotation = "generate.kyverno.io/provenance"
	// PolicyUIDAnnotation binds a controller-owned UpdateRequest to the exact
	// policy whose evaluation authorized it, including across async handoff.
	PolicyUIDAnnotation = "generate.kyverno.io/policy-uid"
	// CleanupPolicyUIDAnnotation preserves the deleted policy identity on a
	// controller-owned cleanup UpdateRequest in the protected installation
	// namespace. It must only be set after authenticating its downstreams.
	CleanupPolicyUIDAnnotation = "generate.kyverno.io/cleanup-policy-uid"

	keyField      = "key"
	keySize       = 32
	versionPrefix = "v1:"
	labelPrefix   = "generate.kyverno.io/"
	domain        = "kyverno.io/legacy-generate-provenance/v1\x00"
)

// EnsureKey provisions the installation key once. An existing invalid key is an
// error, rather than a reason to replace the key and invalidate existing stamps.
func EnsureKey(ctx context.Context, secrets corev1typed.SecretInterface) error {
	if isNil(secrets) {
		return errors.New("generate provenance secret client is unavailable")
	}
	secret, err := secrets.Get(ctx, SecretName, metav1.GetOptions{})
	if err == nil {
		_, err = validateKey(secret)
		return err
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get generate provenance key: %w", err)
	}

	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("create generate provenance key: %w", err)
	}
	immutable := true
	secret, err = secrets.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: SecretName},
		Type:       corev1.SecretTypeOpaque,
		Immutable:  &immutable,
		Data:       map[string][]byte{keyField: key},
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		// Another admission-controller replica won initialization. All replicas
		// must use the persisted winner, never their own discarded random key.
		secret, err = secrets.Get(ctx, SecretName, metav1.GetOptions{})
	}
	if err != nil {
		return fmt.Errorf("provision generate provenance key: %w", err)
	}
	_, err = validateKey(secret)
	return err
}

// Store reads the same installation key in every controller replica. It does
// not provision or rotate keys while handling resource events.
type Store struct {
	secrets     corev1typed.SecretInterface
	keySnapshot []byte
}

// NewStore binds signing and verification to the Kyverno installation namespace.
// A missing client or installation namespace produces a store that fails closed.
func NewStore(client kubernetes.Interface) *Store {
	if isNil(client) || config.KyvernoNamespace() == "" {
		return &Store{}
	}
	return &Store{secrets: client.CoreV1().Secrets(config.KyvernoNamespace())}
}

// Snapshot validates the key before a bounded generation batch writes targets.
// The returned store belongs only to that batch; long-lived controllers retain
// the original store so subsequent batches read the current installation key.
func (s *Store) Snapshot(ctx context.Context) (*Store, error) {
	key, err := s.readKey(ctx)
	if err != nil {
		return nil, err
	}
	return &Store{keySnapshot: append([]byte(nil), key...)}, nil
}

// Sign authenticates the actual persisted resource identity and its controller
// routing labels. Callers must only sign resources whose creation or update was
// authorized independently of resource-supplied generate metadata.
func (s *Store) Sign(ctx context.Context, policy kyvernov1.PolicyInterface, obj *unstructured.Unstructured) (string, error) {
	payload, err := encodePayload(policy, obj)
	if err != nil {
		return "", err
	}
	key, err := s.readKey(ctx)
	if err != nil {
		return "", err
	}
	return versionPrefix + base64.RawURLEncoding.EncodeToString(authenticate(key, payload)), nil
}

// Verify rejects absent, malformed, or stale stamps without treating labels as
// provenance. Key lookup failures are returned so callers also fail closed.
func (s *Store) Verify(ctx context.Context, policy kyvernov1.PolicyInterface, obj *unstructured.Unstructured) (bool, error) {
	if obj == nil {
		return false, nil
	}
	stamp := obj.GetAnnotations()[Annotation]
	encoded, ok := strings.CutPrefix(stamp, versionPrefix)
	if !ok || len(encoded) != base64.RawURLEncoding.EncodedLen(sha256.Size) {
		return false, nil
	}
	mac, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(mac) != sha256.Size {
		return false, nil
	}
	payload, err := encodePayload(policy, obj)
	if err != nil {
		return false, nil
	}
	key, err := s.readKey(ctx)
	if err != nil {
		return false, err
	}
	return hmac.Equal(mac, authenticate(key, payload)), nil
}

func (s *Store) readKey(ctx context.Context) ([]byte, error) {
	if s != nil && s.keySnapshot != nil {
		return s.keySnapshot, nil
	}
	if s == nil || isNil(s.secrets) {
		return nil, errors.New("generate provenance secret client is unavailable")
	}
	secret, err := s.secrets.Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("read generate provenance key: %w", err)
	}
	return validateKey(secret)
}

func validateKey(secret *corev1.Secret) ([]byte, error) {
	if secret == nil || secret.Name != SecretName || secret.Type != corev1.SecretTypeOpaque || secret.Immutable == nil || !*secret.Immutable || len(secret.Data[keyField]) != keySize {
		return nil, errors.New("generate provenance key must be an immutable Opaque Secret with a 32-byte key")
	}
	return secret.Data[keyField], nil
}

func authenticate(key, payload []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(domain))
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

type identity struct {
	Group     string `json:"group,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

type provenancePayload struct {
	Resource identity          `json:"resource"`
	Policy   identity          `json:"policy"`
	Labels   map[string]string `json:"labels"`
}

func encodePayload(policy kyvernov1.PolicyInterface, obj *unstructured.Unstructured) ([]byte, error) {
	if isNil(policy) || obj == nil || policy.GetName() == "" || policy.GetUID() == "" || policy.IsNamespaced() != (policy.GetNamespace() != "") {
		return nil, errors.New("generate provenance requires a persisted policy with a valid scope")
	}
	gv, err := schema.ParseGroupVersion(obj.GetAPIVersion())
	if err != nil || gv.Version == "" || obj.GetKind() == "" || obj.GetName() == "" || obj.GetUID() == "" {
		return nil, errors.New("generate provenance requires a persisted target identity")
	}
	labels := obj.GetLabels()
	if labels[kyverno.LabelAppManagedBy] != kyverno.ValueKyvernoApp || labels[common.GeneratePolicyLabel] != policy.GetName() || labels[common.GeneratePolicyNamespaceLabel] != policy.GetNamespace() || labels[common.GenerateRuleLabel] == "" {
		return nil, errors.New("generate provenance requires matching policy and rule labels")
	}
	if labels[common.GenerateTriggerUIDLabel] == "" || labels[common.GenerateTriggerVersionLabel] == "" || labels[common.GenerateTriggerKindLabel] == "" {
		return nil, errors.New("generate provenance requires a complete trigger identity")
	}
	boundLabels := map[string]string{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp}
	for key, value := range labels {
		if strings.HasPrefix(key, labelPrefix) && key != common.GenerateTypeCloneSourceLabel {
			boundLabels[key] = value
		}
	}
	// A served API version is a representation of the same object, not part of
	// its persistent identity. Bind group, kind and UID so updates through another
	// served version preserve provenance without allowing cross-group replay.
	// encoding/json sorts string map keys, making this encoding independent of
	// Go map iteration and Kubernetes metadata key ordering.
	return json.Marshal(provenancePayload{
		Resource: identity{Group: gv.Group, Kind: obj.GetKind(), Namespace: obj.GetNamespace(), Name: obj.GetName(), UID: string(obj.GetUID())},
		Policy:   identity{Namespace: policy.GetNamespace(), Name: policy.GetName(), UID: string(policy.GetUID())},
		Labels:   boundLabels,
	})
}

func isNil(value any) bool {
	return value == nil || (reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil())
}
