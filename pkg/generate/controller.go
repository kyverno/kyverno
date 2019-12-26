package generate

import (
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
)

type GRController struct {
	// dyanmic client implementation
	client *dclient.Client
	// typed client for kyverno CRDs
	kyvernoClient *kyvernoclient.Clientset
	// event generator interface
	eventGen event.Interface
}
