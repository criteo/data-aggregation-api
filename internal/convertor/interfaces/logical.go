package interfaces

import (
	"fmt"
	"sort"

	cmdbif "github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/rs/zerolog/log"
)

// subinterfacesToOpenconfig converts the logical interfaces of a device
// interface, keyed by index. It returns nil when there is none.
// loopback tells whether the device interface is a loopback.
func subinterfacesToOpenconfig(hostname string, iface *cmdbif.Interface, loopback bool) (map[uint32]*openconfig.Interface_Subinterface, error) {
	if len(iface.LogicalInterfaces) == 0 {
		return nil, nil //nolint:nilnil // no logical interface in the CMDB: nothing to emit
	}

	subinterfaces := make(map[uint32]*openconfig.Interface_Subinterface, len(iface.LogicalInterfaces))
	for _, logicalInterface := range iface.LogicalInterfaces {
		subinterface, err := SubinterfaceToOpenconfig(hostname, logicalInterface, loopback, iface.ManagementRoutes[*logicalInterface.Index], iface.Neighbor)
		if err != nil {
			return nil, err
		}
		subinterfaces[*logicalInterface.Index] = subinterface
	}
	return subinterfaces, nil
}

// SubinterfaceToOpenconfig converts one CMDB logical interface to an OpenConfig subinterface.
// OpenConfig path: /interfaces/interface/subinterfaces/subinterface/.
//
// loopback tells whether the parent interface is a loopback, managementRoutes
// are the management routes attached to the logical interface: the forced
// ones are emitted on its IPv4 container. neighbor is the neighbor of the
// parent interface, nil if none: it resolves the description (see
// Neighbor.ResolveDescription).
func SubinterfaceToOpenconfig(hostname string, logicalInterface *cmdbif.LogicalInterface, loopback bool, managementRoutes []*cmdbif.ManagementRoute,
	neighbor *cmdbif.Neighbor,
) (*openconfig.Interface_Subinterface, error) {
	name := logicalInterface.Name()

	// The MTU of the logical unit 0 is the MTU of the interface, see interfaceMtu.
	var mtu *uint16
	if *logicalInterface.Index != 0 && logicalInterface.Mtu != 0 {
		value, err := mtuToOpenconfig(hostname, logicalInterface)
		if err != nil {
			return nil, err
		}
		mtu = &value
		if logicalInterface.IPv4Address == nil && logicalInterface.IPv6Address == nil {
			log.Warn().Msgf("MTU %d on logical interface %s of %s cannot be emitted: no address family and not unit 0", value, name, hostname)
		}
	}

	subinterface := openconfig.Interface_Subinterface{
		Index:   logicalInterface.Index,
		Enabled: logicalInterface.Enabled,
	}

	if logicalInterface.Description != "" {
		description := neighbor.ResolveDescription(logicalInterface.Description)
		subinterface.Description = &description
	}

	if logicalInterface.IPv4Address != nil {
		forcedRoutes, err := forcedRoutesToOpenconfig(hostname, name, managementRoutes)
		if err != nil {
			return nil, err
		}
		ipv4, err := ipv4ToOpenconfig(hostname, name, logicalInterface, loopback, mtu, forcedRoutes)
		if err != nil {
			return nil, err
		}
		subinterface.Ipv4 = ipv4
	}

	if logicalInterface.IPv6Address != nil {
		ipv6, err := ipv6ToOpenconfig(hostname, name, logicalInterface, loopback, mtu)
		if err != nil {
			return nil, err
		}
		subinterface.Ipv6 = ipv6
	}

	return &subinterface, nil
}

// Host prefix lengths, the only ones that make sense on a loopback.
// Loopback addresses are allocated from pools in the IPAM and the CMDB records
// them with the pool mask (/21, /24, ...), which is not what the interface
// must be configured with.
const (
	ipv4HostPrefixLength uint8 = 32
	ipv6HostPrefixLength uint8 = 128
)

// ipv4ToOpenconfig converts the IPv4 address of a CMDB logical interface.
// OpenConfig path: /interfaces/interface/subinterfaces/subinterface/ipv4/.
func ipv4ToOpenconfig(hostname string, name string, logicalInterface *cmdbif.LogicalInterface, loopback bool, mtu *uint16, forcedRoutes []string) (*openconfig.Interface_Subinterface_Ipv4, error) {
	address := logicalInterface.IPv4Address

	if address.Family != 4 || address.Address.IP.To4() == nil || address.Address.Netmask < 0 || address.Address.Netmask > 32 {
		return nil, fmt.Errorf("inconsistent IPv4 address %s (family %d) on logical interface %s of %s", address.Address.String(), address.Family, name, hostname)
	}

	ip := address.Address.IP.String()
	prefixLength := uint8(address.Address.Netmask)
	if loopback {
		prefixLength = ipv4HostPrefixLength
	}

	return &openconfig.Interface_Subinterface_Ipv4{
		Address: map[string]*openconfig.Interface_Subinterface_Ipv4_Address{
			ip: {
				Ip:           &ip,
				PrefixLength: &prefixLength,
			},
		},
		Mtu:          mtu,
		ForcedRoutes: forcedRoutes,
	}, nil
}

// ipv6ToOpenconfig converts the IPv6 address of a CMDB logical interface.
// OpenConfig path: /interfaces/interface/subinterfaces/subinterface/ipv6/.
func ipv6ToOpenconfig(hostname string, name string, logicalInterface *cmdbif.LogicalInterface, loopback bool, mtu *uint16) (*openconfig.Interface_Subinterface_Ipv6, error) {
	address := logicalInterface.IPv6Address

	if address.Family != 6 || address.Address.IP.To4() != nil || address.Address.Netmask < 0 || address.Address.Netmask > 128 {
		return nil, fmt.Errorf("inconsistent IPv6 address %s (family %d) on logical interface %s of %s", address.Address.String(), address.Family, name, hostname)
	}

	ip := address.Address.IP.String()
	prefixLength := uint8(address.Address.Netmask)
	if loopback {
		prefixLength = ipv6HostPrefixLength
	}

	var ipv6Mtu *uint32
	if mtu != nil {
		value := uint32(*mtu)
		ipv6Mtu = &value
	}

	return &openconfig.Interface_Subinterface_Ipv6{
		Address: map[string]*openconfig.Interface_Subinterface_Ipv6_Address{
			ip: {
				Ip:           &ip,
				PrefixLength: &prefixLength,
			},
		},
		Mtu: ipv6Mtu,
	}, nil
}

// switchedVlanToOpenconfig converts the 802.1Q attributes of a CMDB logical interface.
// OpenConfig path: /interfaces/interface/ethernet/switched-vlan/ (or aggregation/switched-vlan/).
// It returns nil when the logical interface has no 802.1Q mode.
func switchedVlanToOpenconfig(hostname string, logicalInterface *cmdbif.LogicalInterface) (*openconfig.Interface_Ethernet_SwitchedVlan, error) {
	name := logicalInterface.Name()

	switch logicalInterface.Mode {
	case "":
		if logicalInterface.UntaggedVlan != nil || len(logicalInterface.TaggedVlans) > 0 || logicalInterface.NativeVlan != nil {
			log.Warn().Msgf("VLANs on logical interface %s of %s are ignored: no 802.1Q mode set", name, hostname)
		}
		return nil, nil //nolint:nilnil // no 802.1Q configuration on this logical interface

	case cmdbif.ModeAccess:
		if logicalInterface.UntaggedVlan == nil {
			return nil, fmt.Errorf("no untagged VLAN on access mode logical interface %s of %s", name, hostname)
		}
		return &openconfig.Interface_Ethernet_SwitchedVlan{
			InterfaceMode: openconfig.VlanTypes_VlanModeType_ACCESS,
			AccessVlan:    &logicalInterface.UntaggedVlan.Vid,
		}, nil

	case cmdbif.ModeTagged:
		switchedVlan := openconfig.Interface_Ethernet_SwitchedVlan{
			InterfaceMode: openconfig.VlanTypes_VlanModeType_TRUNK,
		}

		if logicalInterface.NativeVlan != nil {
			switchedVlan.NativeVlan = &logicalInterface.NativeVlan.Vid
		}

		if len(logicalInterface.TaggedVlans) == 0 && logicalInterface.NativeVlan == nil {
			log.Warn().Msgf("no tagged nor native VLAN on tagged mode logical interface %s of %s", name, hostname)
		}

		// sorted and deduplicated to keep the output deterministic
		vids := make([]uint16, 0, len(logicalInterface.TaggedVlans))
		seen := make(map[uint16]struct{}, len(logicalInterface.TaggedVlans))
		for _, vlan := range logicalInterface.TaggedVlans {
			if _, duplicate := seen[vlan.Vid]; duplicate {
				continue
			}
			seen[vlan.Vid] = struct{}{}
			vids = append(vids, vlan.Vid)
		}
		sort.Slice(vids, func(i, j int) bool { return vids[i] < vids[j] })

		for _, vid := range vids {
			switchedVlan.TrunkVlans = append(switchedVlan.TrunkVlans, openconfig.UnionUint16(vid))
		}

		return &switchedVlan, nil

	default:
		return nil, fmt.Errorf("unsupported 802.1Q mode %q on logical interface %s of %s", logicalInterface.Mode, name, hostname)
	}
}
