package bgp

import (
	"fmt"

	"github.com/criteo/data-aggregation-api/internal/model/cmdb/bgp"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
)

var safiToOCSafi = map[bgp.AfiSafiChoice]openconfig.E_BgpTypes_AFI_SAFI_TYPE{
	bgp.IPv4Unicast: openconfig.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
	bgp.IPv6Unicast: openconfig.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
	bgp.L2vpnEvpn:   openconfig.BgpTypes_AFI_SAFI_TYPE_L2VPN_EVPN,
}

// BGPGlobalToOpenconfig converts precomputed prefix-lists to OpenConfig.
// OpenConfig path: /network-instances/network-instance/protocols/protocol/bgp/global/.
func BGPGlobalToOpenconfig(bgpGlobal *bgp.BGPGlobal) (*openconfig.NetworkInstance_Protocol_Bgp_Global, error) {
	// TODO: remove once BGP global configuration will be mandatory
	if bgpGlobal == nil {
		return nil, nil
	}

	var safis = make(map[openconfig.E_BgpTypes_AFI_SAFI_TYPE]*openconfig.NetworkInstance_Protocol_Bgp_Global_AfiSafi)
	for _, safi := range bgpGlobal.AfiSafis {
		safiName, ok := safiToOCSafi[safi.Name]
		if !ok {
			return nil, fmt.Errorf("unsupported SAFI: %s", safi.Name)
		}
		newSafi := &openconfig.NetworkInstance_Protocol_Bgp_Global_AfiSafi{AfiSafiName: safiName}

		if len(safi.Aggregates) > 0 {
			aggregates := []string{}
			for _, aggregate := range safi.Aggregates {
				aggregates = append(aggregates, aggregate.Prefix)
			}
			newSafi.Aggregates = aggregates
		}

		if len(safi.RedistributedNetworks) > 0 {
			networks := []string{}
			for _, network := range safi.RedistributedNetworks {
				networks = append(networks, network.Prefix)
			}
			newSafi.RedistributedNetworks = networks
		}

		safis[safiName] = newSafi
	}

	cfg := openconfig.NetworkInstance_Protocol_Bgp_Global{
		As: bgpGlobal.LocalAsn.Number,
		DefaultRouteDistance: &openconfig.NetworkInstance_Protocol_Bgp_Global_DefaultRouteDistance{
			ExternalRouteDistance: bgpGlobal.EBGPAdministrativeDistance,
			InternalRouteDistance: bgpGlobal.IBGPAdministrativeDistance,
		},
		GracefulRestart: &openconfig.NetworkInstance_Protocol_Bgp_Global_GracefulRestart{
			Enabled:     bgpGlobal.GracefulRestartEnabled,
			RestartTime: bgpGlobal.GracefulRestartTime,
		},
		UseMultiplePaths: &openconfig.NetworkInstance_Protocol_Bgp_Global_UseMultiplePaths{
			Enabled: bgpGlobal.EcmpEnabled,
			Ebgp: &openconfig.NetworkInstance_Protocol_Bgp_Global_UseMultiplePaths_Ebgp{
				MaximumPaths: bgpGlobal.EcmpMaximumPaths,
			},
			Ibgp: &openconfig.NetworkInstance_Protocol_Bgp_Global_UseMultiplePaths_Ibgp{
				MaximumPaths: bgpGlobal.EcmpMaximumPaths,
			},
		},
		AfiSafi: safis,
	}

	if bgpGlobal.RouterID != "" {
		cfg.RouterId = &bgpGlobal.RouterID
	}

	return &cfg, nil
}
