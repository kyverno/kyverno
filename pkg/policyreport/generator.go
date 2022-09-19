package policyreport

// import (
// 	"time"

// 	"github.com/go-logr/logr"
// 	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
// 	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
// 	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
// 	"k8s.io/client-go/tools/cache"
// )

// // Generator provides API to create PVs
// type Generator interface {
// 	Add(infos ...Info)
// }

// // generator creates report request
// type generator struct {
// 	cpolLister kyvernov1listers.ClusterPolicyLister
// 	polLister  kyvernov1listers.PolicyLister

// 	informersSynced []cache.InformerSynced

// 	requestCreator creator

// 	log logr.Logger
// }

// // NewReportChangeRequestGenerator returns a new instance of report request generator
// func NewReportChangeRequestGenerator(client versioned.Interface,
// 	cpolInformer kyvernov1informers.ClusterPolicyInformer,
// 	polInformer kyvernov1informers.PolicyInformer,
// 	log logr.Logger,
// ) Generator {
// 	gen := generator{
// 		cpolLister:     cpolInformer.Lister(),
// 		polLister:      polInformer.Lister(),
// 		requestCreator: newChangeRequestCreator(client, 3*time.Second, log.WithName("requestCreator")),
// 		log:            log,
// 	}
// 	gen.informersSynced = []cache.InformerSynced{cpolInformer.Informer().HasSynced, polInformer.Informer().HasSynced}
// 	return &gen
// }

// // Add queues a policy violation create request
// func (gen *generator) Add(infos ...Info) {
// 	for _, info := range infos {
// 		builder := NewBuilder(gen.cpolLister, gen.polLister)
// 		reportReq, err := builder.build(info)
// 		if err != nil {
// 			gen.log.Error(err, "failed to build report")
// 			// return fmt.Errorf("unable to build reportChangeRequest: %v", err)
// 		}
// 		// if reportReq == nil {
// 		// 	return nil
// 		// }
// 		if err == nil {
// 			if err := gen.requestCreator.create(reportReq); err != nil {
// 				gen.log.Error(err, "failed to create report")
// 			}
// 		}
// 	}
// }
