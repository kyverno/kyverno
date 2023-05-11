package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

const (
	resyncPeriod = 15 * time.Minute
)

type RegistryClientLoaderFactory = func(imagePullSecrets string, allowInsecureRegistry bool, registryCredentialHelpers string) RegistryClientLoader

type RegistryClientLoader interface {
	Load(
		ctx context.Context,
		imageVerify kyvernov1.ImageVerification,
		// policyContext PolicyContext,
	) registryclient.Client
	GetGlobalRegistryClient() registryclient.Client
	SetGlobalRegistryClient(rclient registryclient.Client)
}

type registryClientLoader struct {
	logger                logr.Logger
	kubeClient            kubernetes.Interface
	defaultRegistryClient registryclient.Client
}

func DefaultRegistryClientLoaderFactory(ctx context.Context, kubeClient kubernetes.Interface) RegistryClientLoaderFactory {
	return func(imagePullSecrets string, allowInsecureRegistry bool, registryCredentialHelpers string) RegistryClientLoader {
		logger := logging.WithName("registry-client")
		registryClient := setupRegistryClient(ctx, logger, kubeClient, imagePullSecrets, allowInsecureRegistry, registryCredentialHelpers)
		return &registryClientLoader{
			logger:                logger,
			kubeClient:            kubeClient,
			defaultRegistryClient: registryClient,
		}
	}
}

func (rcl *registryClientLoader) Load(
	ctx context.Context,
	imageVerify kyvernov1.ImageVerification,
	// policyContext PolicyContext,
) registryclient.Client {
	if len(imageVerify.ImageRegistryCredentials.Secrets) == 0 {
		checkError(rcl.logger, errors.New("secrets not found"), "secrets not found")
	}
	secrets := make([]string, len(imageVerify.ImageRegistryCredentials.Secrets))
	for i, secret := range imageVerify.ImageRegistryCredentials.Secrets {
		secrets[i] = secret.Name
	}

	helpers := make([]string, len(imageVerify.ImageRegistryCredentials.Helpers))
	for i, helper := range imageVerify.ImageRegistryCredentials.Helpers {
		helpers[i] = string(helper)
	}
	registryCredentialHelpers := strings.Join(helpers, ",")

	return setupRegistryClient(ctx, rcl.logger, rcl.kubeClient, strings.Join(secrets, ","), false, registryCredentialHelpers)
}

func (rcl *registryClientLoader) GetGlobalRegistryClient() registryclient.Client {
	return rcl.defaultRegistryClient
}

func (rcl *registryClientLoader) SetGlobalRegistryClient(rclient registryclient.Client) {
	rcl.defaultRegistryClient = rclient
}

func setupRegistryClient(
	ctx context.Context,
	logger logr.Logger,
	kubeClient kubernetes.Interface,
	imagePullSecrets string,
	allowInsecureRegistry bool,
	registryCredentialHelpers string) registryclient.Client {
	logger = logger.WithName("registry-client").WithValues("secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	logger.Info("setup registry client...")
	registryOptions := []registryclient.Option{
		registryclient.WithTracing(),
	}
	secrets := strings.Split(imagePullSecrets, ",")
	if imagePullSecrets != "" && len(secrets) > 0 {
		factory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
		secretLister := factory.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
		// start informers and wait for cache sync
		factory.Start(ctx.Done())
		for t, result := range factory.WaitForCacheSync(ctx.Done()) {
			if !result {
				checkError(logger, fmt.Errorf("failed to wait for cache sync %T", t), "")
			}
		}
		registryOptions = append(registryOptions, registryclient.WithKeychainPullSecrets(ctx, secretLister, secrets...))
	}
	if allowInsecureRegistry {
		registryOptions = append(registryOptions, registryclient.WithAllowInsecureRegistry())
	}
	if len(registryCredentialHelpers) > 0 {
		registryOptions = append(registryOptions, registryclient.WithCredentialHelpers(strings.Split(registryCredentialHelpers, ",")...))
	}
	registryClient, err := registryclient.New(registryOptions...)
	if err != nil {
		return nil
	}
	return registryClient
}

func checkError(logger logr.Logger, err error, msg string, keysAndValues ...interface{}) {
	if err != nil {
		logger.Error(err, msg, keysAndValues...)
		os.Exit(1)
	}
}
