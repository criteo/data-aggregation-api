package bgp

import (
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/bgp"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/criteo/data-aggregation-api/internal/util"
)

// getBGPsides finds which side of the BGP session we are configuring.
func getBGPsides(hostname string, session *bgp.Session) (*bgp.DeviceSession, *bgp.DeviceSession) {
	var localInfo, remoteInfo bgp.DeviceSession

	if session.PeerA.Device.Name == hostname {
		localInfo = session.PeerA
		remoteInfo = session.PeerB
	} else {
		localInfo = session.PeerB
		remoteInfo = session.PeerA
	}

	return &localInfo, &remoteInfo
}

// NeighborsToOpenconfig converts all BGP neighbors to OpenConfig.
// OpenConfig path: /network-instances/network-instance/protocols/protocol/bgp/neighbors/.
func NeighborsToOpenconfig(hostname string, sessions []*bgp.Session) (map[string]*openconfig.NetworkInstance_Protocol_Bgp_Neighbor, error) {
	var neighbors = make(map[string]*openconfig.NetworkInstance_Protocol_Bgp_Neighbor)

	for _, session := range sessions {
		localInfo, remoteInfo := getBGPsides(hostname, session)

		neighborAddress := remoteInfo.LocalAddress.Address.IP.String()

		policy := openconfig.NetworkInstance_Protocol_Bgp_Neighbor_ApplyPolicy{}
		if localInfo.RoutePolicyIn != nil {
			policy.ImportPolicy = util.AppendIfDefined(policy.ImportPolicy, localInfo.RoutePolicyIn.Name)
		}

		if localInfo.RoutePolicyOut != nil {
			policy.ExportPolicy = util.AppendIfDefined(policy.ExportPolicy, localInfo.RoutePolicyOut.Name)
		}

		safis, err := SafiToOpenconfig(localInfo)
		if err != nil {
			return nil, err
		}

		neighbor := openconfig.NetworkInstance_Protocol_Bgp_Neighbor{
			NeighborAddress: &neighborAddress,
			ApplyPolicy:     &policy,
			AfiSafi:         safis,
			Enabled:         localInfo.Enabled,
			PeerAs:          remoteInfo.LocalAsn.Number,
			LocalAs:         localInfo.LocalAsn.Number,
			AuthPassword:    &session.Password,
			Description:     &localInfo.Description,
			Timers:          &openconfig.NetworkInstance_Protocol_Bgp_Neighbor_Timers{DelayOpenTimer: &localInfo.DelayOpenTimer},
		}

		if localInfo.PeerGroup != nil && localInfo.PeerGroup.Name != "" {
			neighbor.PeerGroup = &localInfo.PeerGroup.Name
		}

		neighbors[neighborAddress] = &neighbor
	}

	return neighbors, nil
}
