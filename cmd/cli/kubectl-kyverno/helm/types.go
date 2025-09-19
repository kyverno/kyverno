package helm

import (
    "time"
    "helm.sh/helm/v3/pkg/chart"
)

// HelmConfig holds configuration for Helm chart processing
type HelmConfig struct {
    ChartPath     string
    ValuesFiles   []string
    SetValues     []string
    SetStringValues []string
    Namespace     string
    ReleaseName   string
    IncludeCRDs   bool
    Validate      bool
    Timeout       time.Duration
}

// RenderedResource represents a rendered Kubernetes resource from Helm chart
type RenderedResource struct {
    Name     string
    Kind     string
    Content  string
    Source   string // Source template file
}

// ChartResult contains the results of chart rendering
type ChartResult struct {
    Chart     *chart.Chart
    Resources []RenderedResource
    Hooks     []RenderedResource
    Notes     string
}
