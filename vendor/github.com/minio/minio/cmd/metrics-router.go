/*
 * MinIO Cloud Storage, (C) 2018 MinIO, Inc.
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
	"os"
	"strings"

	"github.com/gorilla/mux"
)

const (
	prometheusMetricsPath = "/prometheus/metrics"
)

// Standard env prometheus auth type
const (
	EnvPrometheusAuthType = "MINIO_PROMETHEUS_AUTH_TYPE"
)

type prometheusAuthType string

const (
	prometheusJWT    prometheusAuthType = "jwt"
	prometheusPublic prometheusAuthType = "public"
)

// registerMetricsRouter - add handler functions for metrics.
func registerMetricsRouter(router *mux.Router) {
	// metrics router
	metricsRouter := router.NewRoute().PathPrefix(minioReservedBucketPath).Subrouter()

	authType := strings.ToLower(os.Getenv(EnvPrometheusAuthType))
	switch prometheusAuthType(authType) {
	case prometheusPublic:
		metricsRouter.Handle(prometheusMetricsPath, metricsHandler())
	default:
		metricsRouter.Handle(prometheusMetricsPath, AuthMiddleware(metricsHandler()))
	}
}
