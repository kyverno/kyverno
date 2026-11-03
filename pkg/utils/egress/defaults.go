package egress

// The shipped denylists. The copy operators edit lives in charts/kyverno/values.yaml under
// config.registryEgressDeniedHosts / config.registryEgressDeniedIPs; these are the fallback for
// an install with no ConfigMap entry. TestDefaultDenylistsMatchTheChart fails if they drift.
//
// Nothing here is privileged -- every entry can be removed by an operator.

// DefaultDeniedHosts are cloud metadata names, denied by name as well as by address because a
// name is what an image reference carries and the address it resolves to is the provider's.
var DefaultDeniedHosts = []string{
	"metadata.internal",
	"metadata.google.internal",
	"instance-data.ec2.internal",
}

// DefaultDeniedIPs spells out every denied range: there is no implicit "private" rule behind it.
// The third group is the IANA special-purpose registries' "Globally Reachable: False" entries --
// derived wholesale rather than appended one range at a time, which is how a gap like fec0::/10
// survived earlier review rounds. Globally reachable assignments are deliberately absent and
// pinned by TestGloballyReachableRangesAreNotDenied.
var DefaultDeniedIPs = []string{
	// Loopback, unspecified, link-local (incl. 169.254.169.254), multicast.
	"127.0.0.0/8",
	"::1/128",
	"0.0.0.0/8",
	"::/128",
	"169.254.0.0/16",
	"fe80::/10",
	"224.0.0.0/4",
	"ff00::/8",

	// Cluster-local and cloud provider metadata.
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
	"100.100.100.200/32",
	"168.63.129.16/32",
	"fd00:ec2::254/128",

	// IANA special-purpose assignments marked not globally reachable.
	"100.64.0.0/10",
	"198.18.0.0/15",
	"192.0.0.0/24",
	"192.88.99.0/24",
	"240.0.0.0/4",
	"2001::/23",
	"100::/64",
	"192.0.2.0/24",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"2001:db8::/32",
	"3fff::/20",
	"fec0::/10",
	"5f00::/16",
	"64:ff9b:1::/48",
}
