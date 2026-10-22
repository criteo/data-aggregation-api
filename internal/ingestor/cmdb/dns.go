package cmdb

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/ingestor/netbox"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/dns"
)

// GetDNS returns all DNS configuration from the Network CMDB.
func GetDNS() ([]*dns.DNS, error) {
	response := netbox.NetboxResponse[dns.DNS]{}
	params := deviceDatacenterFilter()

	err := netbox.Get("/api/plugins/cmdb/dns/", &response, params)
	if err != nil {
		return nil, fmt.Errorf("DNS fetching failure: %w", err)
	}

	if len(response.Results) == 0 {
		log.Warn().Msg("no DNS configuration found")
	}

	return response.Results, nil
}

// PrecomputeDNS associates each found DNS configuration to the matching devices.
func PrecomputeDNS(configs []*dns.DNS) map[string]*dns.DNS {
	var dnsPerDevice = make(map[string]*dns.DNS)
	for _, config := range configs {
		dnsPerDevice[config.Device.Name] = config
	}

	return dnsPerDevice
}
