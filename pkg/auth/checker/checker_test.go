package checker

import (
	"context"
	"errors"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mock for testing self subject access reviews
type mockSelfClient struct {
	resp *authorizationv1.SelfSubjectAccessReview
	err  error
}

func (m *mockSelfClient) Create(ctx context.Context, review *authorizationv1.SelfSubjectAccessReview, opts metav1.CreateOptions) (*authorizationv1.SelfSubjectAccessReview, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

// mock for testing subject access reviews
type mockSubjectClient struct {
	resp        *authorizationv1.SubjectAccessReview
	err         error
	lastRequest *authorizationv1.SubjectAccessReview
}

func (m *mockSubjectClient) Create(ctx context.Context, review *authorizationv1.SubjectAccessReview, opts metav1.CreateOptions) (*authorizationv1.SubjectAccessReview, error) {
	m.lastRequest = review
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func TestSelfChecker(t *testing.T) {
	// test basic allowed case
	t.Run("allowed", func(t *testing.T) {
		mock := &mockSelfClient{
			resp: &authorizationv1.SelfSubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{
					Allowed: true,
					Reason:  "RBAC: allowed",
				},
			},
		}
		c := NewSelfChecker(mock)
		res, err := c.Check(context.Background(), "", "v1", "pods", "", "default", "", "get")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Allowed {
			t.Error("expected allowed=true")
		}
		if res.Reason != "RBAC: allowed" {
			t.Errorf("got reason %q, want %q", res.Reason, "RBAC: allowed")
		}
	})

	// denied case
	t.Run("denied", func(t *testing.T) {
		mock := &mockSelfClient{
			resp: &authorizationv1.SelfSubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{
					Allowed: false,
					Reason:  "RBAC: no permissions",
				},
			},
		}
		c := NewSelfChecker(mock)
		res, err := c.Check(context.Background(), "", "v1", "secrets", "", "kube-system", "", "delete")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Allowed {
			t.Error("should not be allowed")
		}
	})

	// api error
	t.Run("client error", func(t *testing.T) {
		mock := &mockSelfClient{err: errors.New("connection refused")}
		c := NewSelfChecker(mock)
		_, err := c.Check(context.Background(), "", "v1", "pods", "", "default", "", "get")
		if err == nil {
			t.Error("expected error but got nil")
		}
	})

	// subresource (like pods/log)
	t.Run("subresource", func(t *testing.T) {
		mock := &mockSelfClient{
			resp: &authorizationv1.SelfSubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
			},
		}
		c := NewSelfChecker(mock)
		res, err := c.Check(context.Background(), "", "v1", "pods", "log", "default", "", "get")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Allowed {
			t.Error("should be allowed for subresource")
		}
	})

	// cluster scoped resource like namespaces
	t.Run("cluster scoped", func(t *testing.T) {
		mock := &mockSelfClient{
			resp: &authorizationv1.SelfSubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
			},
		}
		c := NewSelfChecker(mock)
		// namespace param is empty for cluster-scoped
		res, err := c.Check(context.Background(), "", "v1", "namespaces", "", "", "", "create")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Allowed {
			t.Error("expected allowed for cluster-scoped resource")
		}
	})

	// custom resource group like kyverno.io
	t.Run("kyverno CRD", func(t *testing.T) {
		mock := &mockSelfClient{
			resp: &authorizationv1.SelfSubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
			},
		}
		c := NewSelfChecker(mock)
		res, err := c.Check(context.Background(), "kyverno.io", "v1", "clusterpolicies", "", "", "", "get")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Allowed {
			t.Error("should be allowed")
		}
	})

	// when there's an evaluation error in the response
	t.Run("evaluation error", func(t *testing.T) {
		mock := &mockSelfClient{
			resp: &authorizationv1.SelfSubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{
					Allowed:         false,
					EvaluationError: "webhook timeout",
				},
			},
		}
		c := NewSelfChecker(mock)
		res, err := c.Check(context.Background(), "", "v1", "pods", "", "default", "", "get")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Allowed {
			t.Error("should not be allowed when there's eval error")
		}
		if res.EvaluationError != "webhook timeout" {
			t.Errorf("expected evaluation error to be set")
		}
	})
}

func TestSubjectChecker(t *testing.T) {
	t.Run("allowed for service account", func(t *testing.T) {
		mock := &mockSubjectClient{
			resp: &authorizationv1.SubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
			},
		}
		c := NewSubjectChecker(mock, "system:serviceaccount:kyverno:kyverno", nil)
		res, err := c.Check(context.Background(), "", "v1", "pods", "", "default", "", "get")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Allowed {
			t.Error("expected allowed")
		}
	})

	t.Run("denied", func(t *testing.T) {
		mock := &mockSubjectClient{
			resp: &authorizationv1.SubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{
					Allowed: false,
					Reason:  "no permission",
				},
			},
		}
		c := NewSubjectChecker(mock, "system:serviceaccount:default:default", nil)
		res, err := c.Check(context.Background(), "", "v1", "secrets", "", "kube-system", "", "get")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if res.Allowed {
			t.Error("should be denied")
		}
	})

	// empty user should return ErrNoServiceAccount
	t.Run("empty user", func(t *testing.T) {
		mock := &mockSubjectClient{}
		c := NewSubjectChecker(mock, "", nil)
		_, err := c.Check(context.Background(), "", "v1", "pods", "", "default", "", "get")
		if err == nil {
			t.Fatal("expected error for empty user")
		}
		if !errors.Is(err, ErrNoServiceAccount) {
			t.Errorf("expected ErrNoServiceAccount, got %v", err)
		}
	})

	t.Run("api error", func(t *testing.T) {
		mock := &mockSubjectClient{err: errors.New("server unavailable")}
		c := NewSubjectChecker(mock, "some-user", nil)
		_, err := c.Check(context.Background(), "", "v1", "pods", "", "", "", "get")
		if err == nil {
			t.Error("expected error")
		}
	})

	// verify groups are passed correctly
	t.Run("with groups", func(t *testing.T) {
		mock := &mockSubjectClient{
			resp: &authorizationv1.SubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
			},
		}
		groups := []string{"system:authenticated", "developers"}
		c := NewSubjectChecker(mock, "alice", groups)
		_, err := c.Check(context.Background(), "", "v1", "deployments", "", "dev", "", "create")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		// make sure the request has the right user
		if mock.lastRequest.Spec.User != "alice" {
			t.Errorf("user not set correctly in request")
		}
	})

	// checking a specific named resource
	t.Run("specific resource name", func(t *testing.T) {
		mock := &mockSubjectClient{
			resp: &authorizationv1.SubjectAccessReview{
				Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
			},
		}
		c := NewSubjectChecker(mock, "admin", nil)
		res, err := c.Check(context.Background(), "", "v1", "secrets", "", "default", "my-secret", "get")
		if err != nil {
			t.Fatal(err)
		}
		if !res.Allowed {
			t.Error("should be allowed")
		}
	})
}

// helper mock that can return different responses on each call
type multiResponseMock struct {
	responses []*authorizationv1.SelfSubjectAccessReview
	calls     int
	err       error
}

func (m *multiResponseMock) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*AuthResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.calls >= len(m.responses) {
		return &AuthResult{Allowed: true}, nil
	}
	resp := m.responses[m.calls]
	m.calls++
	return &AuthResult{
		Allowed:         resp.Status.Allowed,
		Reason:          resp.Status.Reason,
		EvaluationError: resp.Status.EvaluationError,
	}, nil
}

func TestCheckHelper(t *testing.T) {
	// all verbs allowed
	t.Run("all allowed", func(t *testing.T) {
		mock := &multiResponseMock{
			responses: []*authorizationv1.SelfSubjectAccessReview{
				{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}},
				{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}},
				{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}},
			},
		}
		ok, err := Check(context.Background(), mock, "", "v1", "pods", "", "default", "get", "list", "watch")
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("all verbs should be allowed")
		}
	})

	// second verb denied
	t.Run("one denied", func(t *testing.T) {
		mock := &multiResponseMock{
			responses: []*authorizationv1.SelfSubjectAccessReview{
				{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}},
				{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: false}}, // delete denied
			},
		}
		ok, err := Check(context.Background(), mock, "", "v1", "pods", "", "default", "get", "delete")
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("should return false when one verb is denied")
		}
	})

	// first verb fails - should short circuit
	t.Run("short circuit on first denial", func(t *testing.T) {
		mock := &multiResponseMock{
			responses: []*authorizationv1.SelfSubjectAccessReview{
				{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: false}},
			},
		}
		ok, err := Check(context.Background(), mock, "", "v1", "pods", "", "", "delete", "get", "list")
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("should fail on first verb")
		}
		// should only have made 1 call
		if mock.calls != 1 {
			t.Errorf("expected 1 call (short circuit), got %d", mock.calls)
		}
	})

	t.Run("error from client", func(t *testing.T) {
		mock := &multiResponseMock{err: errors.New("network error")}
		_, err := Check(context.Background(), mock, "", "v1", "pods", "", "", "get")
		if err == nil {
			t.Error("expected error")
		}
	})

	// no verbs should just return true
	t.Run("no verbs", func(t *testing.T) {
		mock := &multiResponseMock{}
		ok, err := Check(context.Background(), mock, "", "v1", "pods", "", "default")
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("empty verbs should return true")
		}
	})

	t.Run("single verb", func(t *testing.T) {
		mock := &multiResponseMock{
			responses: []*authorizationv1.SelfSubjectAccessReview{
				{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}},
			},
		}
		ok, err := Check(context.Background(), mock, "", "v1", "configmaps", "", "kube-system", "get")
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("should be allowed")
		}
	})
}
