package bgp

import (
	"fmt"

	"github.com/criteo/data-aggregation-api/internal/model/cmdb/bgp"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
)

// BGPToOpenconfig converts all precomputed assets in BGP/* to OpenConfig.
// OpenConfig path: /network-instances/network-instance/protocols/protocol/bgp/.
func BGPToOpenconfig(hostname string, bgpGlobal *bgp.BGPGlobal, sessions []*bgp.Session, peerGroups []*bgp.PeerGroup) (*openconfig.NetworkInstance_Protocol_Bgp, error) {
	neighbors, err := NeighborsToOpenconfig(hostname, sessions)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Neighbors to Openconfig: %w", err)
	}

	globalConf, err := BGPGlobalToOpenconfig(bgpGlobal)
	if err != nil {
		return nil, fmt.Errorf("failed to convert BGPGlobal to Openconfig: %w", err)
	}

	defaultInstance := openconfig.NetworkInstance_Protocol_Bgp{
		Global:    globalConf,
		Neighbor:  neighbors,
		PeerGroup: PeerGroupsToOpenconfig(peerGroups),
	}

	return &defaultInstance, err
}
