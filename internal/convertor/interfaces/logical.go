package interfaces

import (
	"fmt"
	"sort"

	cmdbif "github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/rs/zerolog/log"
)

// SubinterfaceToOpenconfig converts one CMDB logical interface to an OpenConfig subinterface.
// OpenConfig path: /interfaces/interface/subinterfaces/subinterface/.
func SubinterfaceToOpenconfig(hostname string, logicalInterface *cmdbif.LogicalInterface) (*openconfig.Interface_Subinterface, error) {
	name := logicalInterface.Name()

	subinterface := openconfig.Interface_Subinterface{
		Index:   logicalInterface.Index,
		Enabled: logicalInterface.Enabled,
	}

	if logicalInterface.Description != "" {
		subinterface.Description = &logicalInterface.Description
	}

	if logicalInterface.IPv4Address != nil {
		ipv4, err := ipv4ToOpenconfig(hostname, name, logicalInterface)
		if err != nil {
			return nil, err
		}
		subinterface.Ipv4 = ipv4
	}

	if logicalInterface.IPv6Address != nil {
		ipv6, err := ipv6ToOpenconfig(hostname, name, logicalInterface)
		if err != nil {
			return nil, err
		}
		subinterface.Ipv6 = ipv6
	}

	return &subinterface, nil
}

// ipv4ToOpenconfig converts the IPv4 address of a CMDB logical interface.
// OpenConfig path: /interfaces/interface/subinterfaces/subinterface/ipv4/.
func ipv4ToOpenconfig(hostname string, name string, logicalInterface *cmdbif.LogicalInterface) (*openconfig.Interface_Subinterface_Ipv4, error) {
	address := logicalInterface.IPv4Address

	if address.Family != 4 || address.Address.IP.To4() == nil || address.Address.Netmask < 0 || address.Address.Netmask > 32 {
		return nil, fmt.Errorf("inconsistent IPv4 address %s (family %d) on logical interface %s of %s", address.Address.String(), address.Family, name, hostname)
	}

	ip := address.Address.IP.String()
	prefixLength := uint8(address.Address.Netmask)

	return &openconfig.Interface_Subinterface_Ipv4{
		Address: map[string]*openconfig.Interface_Subinterface_Ipv4_Address{
			ip: {
				Ip:           &ip,
				PrefixLength: &prefixLength,
			},
		},
	}, nil
}

// ipv6ToOpenconfig converts the IPv6 address of a CMDB logical interface.
// OpenConfig path: /interfaces/interface/subinterfaces/subinterface/ipv6/.
func ipv6ToOpenconfig(hostname string, name string, logicalInterface *cmdbif.LogicalInterface) (*openconfig.Interface_Subinterface_Ipv6, error) {
	address := logicalInterface.IPv6Address

	if address.Family != 6 || address.Address.IP.To4() != nil || address.Address.Netmask < 0 || address.Address.Netmask > 128 {
		return nil, fmt.Errorf("inconsistent IPv6 address %s (family %d) on logical interface %s of %s", address.Address.String(), address.Family, name, hostname)
	}

	ip := address.Address.IP.String()
	prefixLength := uint8(address.Address.Netmask)

	return &openconfig.Interface_Subinterface_Ipv6{
		Address: map[string]*openconfig.Interface_Subinterface_Ipv6_Address{
			ip: {
				Ip:           &ip,
				PrefixLength: &prefixLength,
			},
		},
	}, nil
}

// Host prefix lengths, the only ones that make sense on a loopback.
const (
	ipv4HostPrefixLength uint8 = 32
	ipv6HostPrefixLength uint8 = 128
)

// applyLoopbackPrefixLength forces the host prefix length on the addresses of
// a loopback subinterface. Loopback addresses are allocated from pools in the
// IPAM and the CMDB records them with the pool mask (/21, /24, ...), which
// is not what the interface must be configured with.
func applyLoopbackPrefixLength(subinterface *openconfig.Interface_Subinterface) {
	if subinterface.Ipv4 != nil {
		for _, address := range subinterface.Ipv4.Address {
			prefixLength := ipv4HostPrefixLength
			address.PrefixLength = &prefixLength
		}
	}
	if subinterface.Ipv6 != nil {
		for _, address := range subinterface.Ipv6.Address {
			prefixLength := ipv6HostPrefixLength
			address.PrefixLength = &prefixLength
		}
	}
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
