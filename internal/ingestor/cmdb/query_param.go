package cmdb

import (
	"net/url"

	"github.com/criteo/data-aggregation-api/internal/config"
	"github.com/rs/zerolog/log"
)

func deviceDatacenterFilter() url.Values {
	return prefixedDeviceDatacenterFilter("")
}

// prefixedDeviceDatacenterFilter returns the datacenter filter for objects
// which are not directly attached to a device (e.g. logical interfaces are
// attached via their parent interface: "parent_interface__device__site__name").
func prefixedDeviceDatacenterFilter(prefix string) url.Values {
	datacenterFilter := ""

	switch string(config.Cfg.NetBox.DatacenterFilterKey) {
	case "site":
		datacenterFilter = "device__site__name"
	case "site_group":
		datacenterFilter = "device__site__group__name"
	case "region":
		datacenterFilter = "device__site__region__name"
	default:
		log.Fatal().Msgf("unknown datacenter filter: %s", config.Cfg.NetBox.DatacenterFilterKey)
	}

	params := url.Values{}
	params.Set(prefix+datacenterFilter, config.Cfg.Datacenter)

	return params
}
