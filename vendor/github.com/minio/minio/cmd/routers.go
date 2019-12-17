/*
 * MinIO Cloud Storage, (C) 2015, 2016 MinIO, Inc.
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
	"net/http"

	"github.com/gorilla/mux"
)

// Composed function registering routers for only distributed XL setup.
func registerDistXLRouters(router *mux.Router, endpointZones EndpointZones) {
	// Register storage REST router only if its a distributed setup.
	registerStorageRESTHandlers(router, endpointZones)

	// Register peer REST router only if its a distributed setup.
	registerPeerRESTHandlers(router)

	// Register bootstrap REST router for distributed setups.
	registerBootstrapRESTHandlers(router)

	// Register distributed namespace lock routers.
	registerLockRESTHandlers(router, endpointZones)
}

// List of some generic handlers which are applied for all incoming requests.
var globalHandlers = []HandlerFunc{
	// set x-amz-request-id header.
	addCustomHeaders,
	// set HTTP security headers such as Content-Security-Policy.
	addSecurityHeaders,
	// Forward path style requests to actual host in a bucket federated setup.
	setBucketForwardingHandler,
	// Validate all the incoming requests.
	setRequestValidityHandler,
	// Network statistics
	setHTTPStatsHandler,
	// Limits all requests size to a maximum fixed limit
	setRequestSizeLimitHandler,
	// Limits all header sizes to a maximum fixed limit
	setRequestHeaderSizeLimitHandler,
	// Adds 'crossdomain.xml' policy handler to serve legacy flash clients.
	setCrossDomainPolicy,
	// Redirect some pre-defined browser request paths to a static location prefix.
	setBrowserRedirectHandler,
	// Validates if incoming request is for restricted buckets.
	setReservedBucketHandler,
	// Adds cache control for all browser requests.
	setBrowserCacheControlHandler,
	// Validates all incoming requests to have a valid date header.
	setTimeValidityHandler,
	// CORS setting for all browser API requests.
	setCorsHandler,
	// Validates all incoming URL resources, for invalid/unsupported
	// resources client receives a HTTP error.
	setIgnoreResourcesHandler,
	// Auth handler verifies incoming authorization headers and
	// routes them accordingly. Client receives a HTTP error for
	// invalid/unsupported signatures.
	setAuthHandler,
	// Enforce rules specific for TLS requests
	setSSETLSHandler,
	// filters HTTP headers which are treated as metadata and are reserved
	// for internal use only.
	filterReservedMetadata,
	// Add new handlers here.
}

// configureServer handler returns final handler for the http server.
func configureServerHandler(endpointZones EndpointZones) (http.Handler, error) {
	// Initialize router. `SkipClean(true)` stops gorilla/mux from
	// normalizing URL path minio/minio#3256
	router := mux.NewRouter().SkipClean(true)

	// Initialize distributed NS lock.
	if globalIsDistXL {
		registerDistXLRouters(router, endpointZones)
	}

	// Add STS router always.
	registerSTSRouter(router)

	// Add Admin router, all APIs are enabled in server mode.
	registerAdminRouter(router, true, true)

	// Add healthcheck router
	registerHealthCheckRouter(router)

	// Add server metrics router
	registerMetricsRouter(router)

	// Register web router when its enabled.
	if globalBrowserEnabled {
		if err := registerWebRouter(router); err != nil {
			return nil, err
		}
	}

	// Add API router, additionally all server mode support encryption
	// but don't allow SSE-KMS.
	registerAPIRouter(router, true, false)

	// If none of the routes match add default error handler routes
	router.NotFoundHandler = http.HandlerFunc(httpTraceAll(errorResponseHandler))
	router.MethodNotAllowedHandler = http.HandlerFunc(httpTraceAll(errorResponseHandler))

	// Register rest of the handlers.
	return registerHandlers(router, globalHandlers...), nil
}
