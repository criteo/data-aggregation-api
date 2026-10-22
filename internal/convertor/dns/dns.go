package dns

import (
	"fmt"
	"slices"

	"github.com/criteo/data-aggregation-api/internal/model/cmdb/dns"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
)

// DNSToOpenconfig converts the DNS nameservers of a device to OpenConfig.
// The servers list is ordered-by user, but the CMDB carries no priority and
// SONiC sorts the nameservers anyway: they are sorted by address to keep the
// output stable.
func DNSToOpenconfig(config *dns.DNS) (*openconfig.System_Dns, error) {
	if config == nil || len(config.ServerList) == 0 {
		return nil, nil
	}

	addresses := make([]string, 0, len(config.ServerList))
	for _, s := range config.ServerList {
		addresses = append(addresses, s.ServerAddress)
	}
	slices.Sort(addresses)
	addresses = slices.Compact(addresses)

	out := &openconfig.System_Dns{}
	for _, address := range addresses {
		if _, err := out.AppendNewServer(address); err != nil {
			return nil, fmt.Errorf("DNS server %s: %w", address, err)
		}
	}

	return out, nil
}
