/*
 * MinIO Cloud Storage, (C) 2017, 2018 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/minio/cli"
	"github.com/minio/minio/cmd/config"
	xhttp "github.com/minio/minio/cmd/http"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/certs"
	"github.com/minio/minio/pkg/color"
	"github.com/minio/minio/pkg/env"
)

func init() {
	logger.Init(GOPATH, GOROOT)
	logger.RegisterError(config.FmtError)
}

var (
	gatewayCmd = cli.Command{
		Name:            "gateway",
		Usage:           "start object storage gateway",
		Flags:           append(ServerFlags, GlobalFlags...),
		HideHelpCommand: true,
	}
)

// RegisterGatewayCommand registers a new command for gateway.
func RegisterGatewayCommand(cmd cli.Command) error {
	cmd.Flags = append(append(cmd.Flags, ServerFlags...), GlobalFlags...)
	gatewayCmd.Subcommands = append(gatewayCmd.Subcommands, cmd)
	return nil
}

// ParseGatewayEndpoint - Return endpoint.
func ParseGatewayEndpoint(arg string) (endPoint string, secure bool, err error) {
	schemeSpecified := len(strings.Split(arg, "://")) > 1
	if !schemeSpecified {
		// Default connection will be "secure".
		arg = "https://" + arg
	}

	u, err := url.Parse(arg)
	if err != nil {
		return "", false, err
	}

	switch u.Scheme {
	case "http":
		return u.Host, false, nil
	case "https":
		return u.Host, true, nil
	default:
		return "", false, fmt.Errorf("Unrecognized scheme %s", u.Scheme)
	}
}

// ValidateGatewayArguments - Validate gateway arguments.
func ValidateGatewayArguments(serverAddr, endpointAddr string) error {
	if err := CheckLocalServerAddr(serverAddr); err != nil {
		return err
	}

	if endpointAddr != "" {
		// Reject the endpoint if it points to the gateway handler itself.
		sameTarget, err := sameLocalAddrs(endpointAddr, serverAddr)
		if err != nil {
			return err
		}
		if sameTarget {
			return fmt.Errorf("endpoint points to the local gateway")
		}
	}
	return nil
}

// StartGateway - handler for 'minio gateway <name>'.
func StartGateway(ctx *cli.Context, gw Gateway) {
	if gw == nil {
		logger.FatalIf(errUnexpected, "Gateway implementation not initialized")
	}

	// Disable logging until gateway initialization is complete, any
	// error during initialization will be shown as a fatal message
	logger.Disable = true

	// Validate if we have access, secret set through environment.
	globalGatewayName = gw.Name()
	gatewayName := gw.Name()
	if ctx.Args().First() == "help" {
		cli.ShowCommandHelpAndExit(ctx, gatewayName, 1)
	}

	// Handle common command args.
	handleCommonCmdArgs(ctx)

	// Get port to listen on from gateway address
	globalMinioHost, globalMinioPort = mustSplitHostPort(globalCLIContext.Addr)

	// On macOS, if a process already listens on LOCALIPADDR:PORT, net.Listen() falls back
	// to IPv6 address ie minio will start listening on IPv6 address whereas another
	// (non-)minio process is listening on IPv4 of given port.
	// To avoid this error situation we check for port availability.
	logger.FatalIf(checkPortAvailability(globalMinioHost, globalMinioPort), "Unable to start the gateway")

	// Check and load TLS certificates.
	var err error
	globalPublicCerts, globalTLSCerts, globalIsSSL, err = getTLSConfig()
	logger.FatalIf(err, "Invalid TLS certificate file")

	// Check and load Root CAs.
	globalRootCAs, err = config.GetRootCAs(globalCertsCADir.Get())
	logger.FatalIf(err, "Failed to read root CAs (%v)", err)

	// Handle gateway specific env
	gatewayHandleEnvVars()

	// Set system resources to maximum.
	logger.LogIf(context.Background(), setMaxResources())

	initNSLock(false) // Enable local namespace lock.

	// Set when gateway is enabled
	globalIsGateway = true

	enableConfigOps := gatewayName == "nas"

	// TODO: We need to move this code with globalConfigSys.Init()
	// for now keep it here such that "s3" gateway layer initializes
	// itself properly when KMS is set.

	// Initialize server config.
	srvCfg := newServerConfig()

	// Override any values from ENVs.
	if err := lookupConfigs(srvCfg); err != nil {
		logger.FatalIf(err, "Unable to initialize server config")
	}

	// hold the mutex lock before a new config is assigned.
	globalServerConfigMu.Lock()
	globalServerConfig = srvCfg
	globalServerConfigMu.Unlock()

	router := mux.NewRouter().SkipClean(true)

	if globalEtcdClient != nil {
		// Enable STS router if etcd is enabled.
		registerSTSRouter(router)
	}

	// Initialize globalConsoleSys system
	globalConsoleSys = NewConsoleLogger(context.Background(), globalEndpoints)
	enableIAMOps := globalEtcdClient != nil

	// Enable IAM admin APIs if etcd is enabled, if not just enable basic
	// operations such as profiling, server info etc.
	registerAdminRouter(router, enableConfigOps, enableIAMOps)

	// Add healthcheck router
	registerHealthCheckRouter(router)

	// Add server metrics router
	registerMetricsRouter(router)

	// Register web router when its enabled.
	if globalBrowserEnabled {
		logger.FatalIf(registerWebRouter(router), "Unable to configure web browser")
	}

	// Currently only NAS and S3 gateway support encryption headers.
	encryptionEnabled := gatewayName == "s3" || gatewayName == "nas"
	allowSSEKMS := gatewayName == "s3" // Only S3 can support SSE-KMS (as pass-through)

	// Add API router.
	registerAPIRouter(router, encryptionEnabled, allowSSEKMS)

	var getCert certs.GetCertificateFunc
	if globalTLSCerts != nil {
		getCert = globalTLSCerts.GetCertificate
	}

	globalHTTPServer = xhttp.NewServer([]string{globalCLIContext.Addr},
		criticalErrorHandler{registerHandlers(router, globalHandlers...)}, getCert)
	go func() {
		globalHTTPServerErrorCh <- globalHTTPServer.Start()
	}()

	signal.Notify(globalOSSignalCh, os.Interrupt, syscall.SIGTERM)

	newObject, err := gw.NewGatewayLayer(globalActiveCred)
	if err != nil {
		// Stop watching for any certificate changes.
		globalTLSCerts.Stop()

		globalHTTPServer.Shutdown()
		logger.FatalIf(err, "Unable to initialize gateway backend")
	}

	// Re-enable logging
	logger.Disable = false

	// Once endpoints are finalized, initialize the new object api in safe mode.
	globalObjLayerMutex.Lock()
	globalSafeMode = true
	globalObjectAPI = newObject
	globalObjLayerMutex.Unlock()

	// Populate existing buckets to the etcd backend
	if globalDNSConfig != nil {
		buckets, err := newObject.ListBuckets(context.Background())
		if err != nil {
			logger.Fatal(err, "Unable to list buckets")
		}
		initFederatorBackend(buckets, newObject)
	}

	// Migrate all backend configs to encrypted backend, also handles rotation as well.
	// For "nas" gateway we need to specially handle the backend migration as well.
	// Internally code handles migrating etcd if enabled automatically.
	logger.FatalIf(handleEncryptedConfigBackend(newObject, enableConfigOps),
		"Unable to handle encrypted backend for config, iam and policies")

	// Calls all New() for all sub-systems.
	newAllSubsystems()

	// ****  WARNING ****
	// Migrating to encrypted backend should happen before initialization of any
	// sub-systems, make sure that we do not move the above codeblock elsewhere.
	if enableConfigOps {
		logger.FatalIf(globalConfigSys.Init(newObject), "Unable to initialize config system")

		// Start watching disk for reloading config, this
		// is only enabled for "NAS" gateway.
		globalConfigSys.WatchConfigNASDisk(newObject)
	}

	// This is only to uniquely identify each gateway deployments.
	globalDeploymentID = env.Get("MINIO_GATEWAY_DEPLOYMENT_ID", mustGetUUID())
	logger.SetDeploymentID(globalDeploymentID)

	if globalEtcdClient != nil {
		// ****  WARNING ****
		// Migrating to encrypted backend on etcd should happen before initialization of
		// IAM sub-systems, make sure that we do not move the above codeblock elsewhere.
		logger.FatalIf(migrateIAMConfigsEtcdToEncrypted(globalEtcdClient),
			"Unable to handle encrypted backend for iam and policies")
	}

	if enableIAMOps {
		// Initialize IAM sys.
		logger.FatalIf(globalIAMSys.Init(newObject), "Unable to initialize IAM system")
	}

	if globalCacheConfig.Enabled {
		// initialize the new disk cache objects.
		var cacheAPI CacheObjectLayer
		cacheAPI, err = newServerCacheObjects(context.Background(), globalCacheConfig)
		logger.FatalIf(err, "Unable to initialize disk caching")

		globalObjLayerMutex.Lock()
		globalCacheObjectAPI = cacheAPI
		globalObjLayerMutex.Unlock()
	}

	// Verify if object layer supports
	// - encryption
	// - compression
	verifyObjectLayerFeatures("gateway "+gatewayName, newObject)

	// Prints the formatted startup message once object layer is initialized.
	if !globalCLIContext.Quiet {
		mode := globalMinioModeGatewayPrefix + gatewayName
		// Check update mode.
		checkUpdate(mode)

		// Print a warning message if gateway is not ready for production before the startup banner.
		if !gw.Production() {
			logStartupMessage(color.Yellow("               *** Warning: Not Ready for Production ***"))
		}

		// Print gateway startup message.
		printGatewayStartupMessage(getAPIEndpoints(), gatewayName)
	}

	// Disable safe mode operation, after all initialization is over.
	globalObjLayerMutex.Lock()
	globalSafeMode = false
	globalObjLayerMutex.Unlock()

	// Set uptime time after object layer has initialized.
	globalBootTime = UTCNow()

	handleSignals()
}
