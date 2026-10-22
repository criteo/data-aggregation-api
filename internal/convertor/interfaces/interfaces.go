package interfaces

import (
	"fmt"
	"math"
	"strconv"

	cmdbif "github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
)

// DefaultNetworkInstance is the name of the default network instance,
// where interfaces without a VRF are bound.
const DefaultNetworkInstance = "default"

// NetworkInstanceInterfaces maps a network instance name to its interface
// bindings, keyed by binding id.
// OpenConfig path: /network-instances/network-instance/interfaces/interface/.
type NetworkInstanceInterfaces map[string]map[string]*openconfig.NetworkInstance_Interface

// InterfacesToOpenconfig converts the precomputed CMDB interfaces of one device to OpenConfig.
// OpenConfig path: /interfaces/interface/.
//
// It also returns the interface to network instance (VRF) bindings: see networkInstanceOf.
func InterfacesToOpenconfig(hostname string, ifaces []*cmdbif.Interface) (map[string]*openconfig.Interface, NetworkInstanceInterfaces, error) {
	interfaces := make(map[string]*openconfig.Interface, len(ifaces))
	bindings := make(NetworkInstanceInterfaces)

	for _, iface := range ifaces {
		converted, err := DeviceInterfaceToOpenconfig(hostname, iface)
		if err != nil {
			return nil, nil, err
		}
		interfaces[iface.Name] = converted

		for _, logicalInterface := range iface.LogicalInterfaces {
			networkInstance, ok := networkInstanceOf(logicalInterface)
			if !ok {
				continue
			}
			if bindings[networkInstance] == nil {
				bindings[networkInstance] = make(map[string]*openconfig.NetworkInstance_Interface)
			}
			binding := networkInstanceBinding(iface.Name, logicalInterface)
			bindings[networkInstance][*binding.Id] = binding
		}
	}

	return interfaces, bindings, nil
}

// unit0 returns the logical unit 0 of an interface, nil if none.
func unit0(iface *cmdbif.Interface) *cmdbif.LogicalInterface {
	for _, logicalInterface := range iface.LogicalInterfaces {
		if *logicalInterface.Index == 0 {
			return logicalInterface
		}
	}
	return nil
}

// mtuToOpenconfig validates the MTU of a logical interface.
func mtuToOpenconfig(hostname string, logicalInterface *cmdbif.LogicalInterface) (uint16, error) {
	if logicalInterface.Mtu > math.MaxUint16 {
		return 0, fmt.Errorf("invalid MTU %d on logical interface %s of %s", logicalInterface.Mtu, logicalInterface.Name(), hostname)
	}
	return uint16(logicalInterface.Mtu), nil
}

// interfaceMtu returns the MTU of an interface. OpenConfig only models the
// MTU on the interface itself and on the IPv4/IPv6 containers of a
// subinterface, so the MTU of the logical unit 0 is the MTU of the interface
// (the MTU of other units is set on their address families). It returns nil
// when unit 0 has no MTU.
func interfaceMtu(hostname string, iface *cmdbif.Interface) (*uint16, error) {
	unit := unit0(iface)
	if unit == nil || unit.Mtu == 0 {
		return nil, nil //nolint:nilnil // no MTU in the CMDB: nothing to emit
	}
	mtu, err := mtuToOpenconfig(hostname, unit)
	if err != nil {
		return nil, err
	}
	return &mtu, nil
}

// ipv6LinkLocalOnly returns the CMDB use_ipv6_link_local_only flag of an
// interface. The CMDB stores the flag on the logical interface but SONiC
// configures it on the interface itself, so only the logical unit 0 carries
// it (enforced by the precompute). The CMDB value is emitted as-is, true or
// false; the leaf is only absent when the CMDB has no value.
func ipv6LinkLocalOnly(iface *cmdbif.Interface) *bool {
	unit := unit0(iface)
	if unit == nil {
		return nil
	}
	return unit.UseIPv6LinkLocalOnly
}

// interfaceSwitchedVlan returns the 802.1Q attributes of an interface, carried
// by at most one of its logical interfaces. It returns nil when none has an
// 802.1Q mode.
func interfaceSwitchedVlan(hostname string, iface *cmdbif.Interface) (*openconfig.Interface_Ethernet_SwitchedVlan, error) {
	var found *openconfig.Interface_Ethernet_SwitchedVlan
	for _, logicalInterface := range iface.LogicalInterfaces {
		switchedVlan, err := switchedVlanToOpenconfig(hostname, logicalInterface)
		if err != nil {
			return nil, err
		}
		if switchedVlan == nil {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("conflicting 802.1Q configurations on device interface %s of %s", iface.DeviceInterface.Name, hostname)
		}
		found = switchedVlan
	}
	return found, nil
}

// toAggregationSwitchedVlan converts an Ethernet switched-vlan to its LAG
// counterpart: the two OpenConfig containers share the same leaves but are
// distinct generated types.
func toAggregationSwitchedVlan(switchedVlan *openconfig.Interface_Ethernet_SwitchedVlan) *openconfig.Interface_Aggregation_SwitchedVlan {
	out := openconfig.Interface_Aggregation_SwitchedVlan{
		InterfaceMode: switchedVlan.InterfaceMode,
		AccessVlan:    switchedVlan.AccessVlan,
		NativeVlan:    switchedVlan.NativeVlan,
	}

	for _, vid := range switchedVlan.TrunkVlans {
		if union, ok := vid.(openconfig.Interface_Aggregation_SwitchedVlan_TrunkVlans_Union); ok {
			out.TrunkVlans = append(out.TrunkVlans, union)
		}
	}

	return &out
}

// networkInstanceOf returns the network instance (VRF) a logical interface is
// bound to: its explicit VRF first, the default network instance for L3
// interfaces without a VRF. It returns false when the logical interface is
// bound to none.
func networkInstanceOf(logicalInterface *cmdbif.LogicalInterface) (string, bool) {
	switch {
	case logicalInterface.Vrf != nil:
		return logicalInterface.Vrf.Name, true
	case logicalInterface.IPv4Address != nil || logicalInterface.IPv6Address != nil:
		return DefaultNetworkInstance, true
	default:
		return "", false
	}
}

// networkInstanceBinding returns the binding of a logical interface in its
// network instance. parentName is the name of the parent interface on the device.
func networkInstanceBinding(parentName string, logicalInterface *cmdbif.LogicalInterface) *openconfig.NetworkInstance_Interface {
	id := parentName + "." + strconv.FormatUint(uint64(*logicalInterface.Index), 10)
	return &openconfig.NetworkInstance_Interface{
		Id:           &id,
		Interface:    &parentName,
		Subinterface: logicalInterface.Index,
	}
}
