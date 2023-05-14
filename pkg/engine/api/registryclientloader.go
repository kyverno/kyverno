package api

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	v1 "k8s.io/client-go/listers/core/v1"
)

type RegistryClientLoaderFactory = func(imagePullSecrets string, allowInsecureRegistry bool, registryCredentialHelpers string) RegistryClientLoader

type RegistryClientLoader interface {
	Load(
		ctx context.Context,
		imageVerify kyvernov1.ImageVerification,
		policyContext PolicyContext,
	) registryclient.Client
	GetGlobalRegistryClient() registryclient.Client
}

type registryClientLoader struct {
	logger                logr.Logger
	secretLister          v1.SecretNamespaceLister
	defaultRegistryClient registryclient.Client
}

func DefaultRegistryClientLoaderFactory(ctx context.Context, secretLister v1.SecretNamespaceLister) RegistryClientLoaderFactory {
	return func(imagePullSecrets string, allowInsecureRegistry bool, registryCredentialHelpers string) RegistryClientLoader {
		logger := logging.WithName("registry-client")
		registryClient := setupRegistryClient(ctx, logger, secretLister, imagePullSecrets, allowInsecureRegistry, registryCredentialHelpers)
		return &registryClientLoader{
			logger:                logger,
			secretLister:          secretLister,
			defaultRegistryClient: registryClient,
		}
	}
}

func RegistryClientLoaderNewOrDie(options ...registryclient.Option) RegistryClientLoader {
	return &registryClientLoader{
		logger:                logging.WithName("registry-client"),
		secretLister:          nil,
		defaultRegistryClient: registryclient.NewOrDie(options...),
	}
}

func (rcl *registryClientLoader) Load(
	ctx context.Context,
	imageVerify kyvernov1.ImageVerification,
	policyContext PolicyContext,
) registryclient.Client {
	if rcl.secretLister == nil { // only nil when a fake registryClientLoader is created
		return rcl.defaultRegistryClient
	}
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

	return setupRegistryClient(ctx, rcl.logger, rcl.secretLister, strings.Join(secrets, ","), false, registryCredentialHelpers)
}

func (rcl *registryClientLoader) GetGlobalRegistryClient() registryclient.Client {
	return rcl.defaultRegistryClient
}

func setupRegistryClient(
	ctx context.Context,
	logger logr.Logger,
	secretLister v1.SecretNamespaceLister,
	imagePullSecrets string,
	allowInsecureRegistry bool,
	registryCredentialHelpers string,
) registryclient.Client {
	logger = logger.WithName("registry-client").WithValues("secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	logger.Info("setup registry client...")
	registryOptions := []registryclient.Option{
		registryclient.WithTracing(),
	}
	secrets := strings.Split(imagePullSecrets, ",")
	if imagePullSecrets != "" && len(secrets) > 0 {
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
