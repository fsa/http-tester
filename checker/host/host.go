package host

import (
	"net"

	"http-tester/checker"
)

// CheckLocalConnectivity checks whether the local machine has IPv4 and IPv6
// connectivity by inspecting network interfaces. Results are saved to stats
// as host-ipv4 / host-ipv6 entries so other checkers can read them.
func CheckLocalConnectivity(stats *checker.Stats) (hasIPv4, hasIPv6 bool) {
	hasIPv4, hasIPv6 = checkInterfaces()

	stats.AddRunResult(checker.RunResult{
		Domain: "",
		Results: []*checker.Result{
			{
				Checker: "host-ipv4",
				Group:   "Host",
				Tags:    []string{"ipv4"},
				Passed:  hasIPv4,
				Details: statusText(hasIPv4, "IPv4"),
			},
			{
				Checker: "host-ipv6",
				Group:   "Host",
				Tags:    []string{"ipv6"},
				Passed:  hasIPv6,
				Details: statusText(hasIPv6, "IPv6"),
			},
		},
	})

	return hasIPv4, hasIPv6
}

func checkInterfaces() (hasIPv4, hasIPv6 bool) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false, false
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if ip.To4() != nil {
				hasIPv4 = true
			} else if ip.To16() != nil {
				hasIPv6 = true
			}
		}
	}
	return hasIPv4, hasIPv6
}

func statusText(available bool, ipVersion string) string {
	if available {
		return ipVersion + " available on this host"
	}
	return ipVersion + " not available on this host"
}
