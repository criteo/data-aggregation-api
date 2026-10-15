package interfaces

import (
	"fmt"
	"math"
	"strconv"

	cmdbif "github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/rs/zerolog/log"
)

// DefaultNetworkInstance is the name of the default network instance,
// where interfaces without a VRF are bound.
const DefaultNetworkInstance = "default"

// NetworkInstanceInterfaces maps a network instance name to its interface
// bindings, keyed by binding id.
// OpenConfig path: /network-instances/network-instance/interfaces/interface/.
type NetworkInstanceInterfaces map[string]map[string]*openconfig.NetworkInstance_Interface

// InterfacesToOpenconfig converts all precomputed CMDB interfaces of one device to OpenConfig.
// OpenConfig path: /interfaces/interface/.
//
// portLayout is the port layout of the device, keyed by CMDB interface name.
// Front-panel ports found in it are renamed to their vendor name and carry
// their alias and lanes; it may be nil. withPortIndex additionally emits the
// front-panel port index, which only AFK managed devices need.
//
// neighbors gives the remote end of the link connected to each device
// interface, keyed by CMDB interface name; it may be nil.
//
// It also returns the interface to network instance (VRF) bindings: a logical
// interface with a VRF is bound to the matching network instance, and a
// logical interface carrying at least one IP address but no VRF is bound to
// the default network instance.
func InterfacesToOpenconfig(hostname string, deviceInterfaces []*cmdbif.DeviceInterface, logicalInterfaces []*cmdbif.LogicalInterface, portLayout cmdbif.PortLayoutTable, neighbors cmdbif.NeighborTable, withPortIndex bool) (map[string]*openconfig.Interface, NetworkInstanceInterfaces, error) {
	interfaces := make(map[string]*openconfig.Interface, len(deviceInterfaces))
	bindings := make(NetworkInstanceInterfaces)

	// OpenConfig interface name per CMDB device interface name
	names := make(map[string]string, len(deviceInterfaces))

	for _, deviceInterface := range deviceInterfaces {
		if _, duplicate := names[deviceInterface.Name]; duplicate {
			return nil, nil, fmt.Errorf("duplicate device interface %s on %s", deviceInterface.Name, hostname)
		}

		port := portLayout[deviceInterface.Name]
		name := InterfaceName(deviceInterface, port)
		if _, duplicate := interfaces[name]; duplicate {
			return nil, nil, fmt.Errorf("device interfaces %s and %s of %s both map to port %s", deviceInterface.Name, cmdbNameOf(names, name), hostname, name)
		}

		iface, err := DeviceInterfaceToOpenconfig(hostname, deviceInterface, port, withPortIndex)
		if err != nil {
			return nil, nil, err
		}
		applyNeighbor(iface, neighbors[deviceInterface.Name])
		interfaces[name] = iface
		names[deviceInterface.Name] = name
	}

	for _, logicalInterface := range logicalInterfaces {
		name := logicalInterface.Name()

		if logicalInterface.Index == nil {
			return nil, nil, fmt.Errorf("missing index on logical interface of %s on %s", logicalInterface.ParentInterface.Name, hostname)
		}

		parentName, ok := names[logicalInterface.ParentInterface.Name]
		if !ok {
			return nil, nil, fmt.Errorf("logical interface %s of %s references unknown device interface %s", name, hostname, logicalInterface.ParentInterface.Name)
		}
		parent := interfaces[parentName]

		index := *logicalInterface.Index
		if _, duplicate := parent.Subinterface[index]; duplicate {
			return nil, nil, fmt.Errorf("duplicate logical interface %s on %s", name, hostname)
		}

		subinterface, err := SubinterfaceToOpenconfig(hostname, logicalInterface)
		if err != nil {
			return nil, nil, err
		}

		if parent.Type == openconfig.IETFInterfaces_InterfaceType_softwareLoopback {
			applyLoopbackPrefixLength(subinterface)
		}

		if parent.Subinterface == nil {
			parent.Subinterface = make(map[uint32]*openconfig.Interface_Subinterface)
		}
		parent.Subinterface[index] = subinterface

		if err := applyMtu(hostname, logicalInterface, parent, subinterface); err != nil {
			return nil, nil, err
		}

		applyIPv6LinkLocalOnly(hostname, logicalInterface, parent)

		if err := applySwitchedVlan(hostname, logicalInterface, parent); err != nil {
			return nil, nil, err
		}

		addNetworkInstanceBinding(bindings, logicalInterface, parentName)
	}

	return interfaces, bindings, nil
}

// cmdbNameOf returns the CMDB device interface name mapped to an OpenConfig
// interface name, for error messages.
func cmdbNameOf(names map[string]string, openconfigName string) string {
	for cmdbName, name := range names {
		if name == openconfigName {
			return cmdbName
		}
	}
	return openconfigName
}

// applyIPv6LinkLocalOnly maps the CMDB use_ipv6_link_local_only flag to the
// afk-interfaces-ext leaf of the parent interface. The CMDB stores the flag on
// the logical interface but SONiC configures it on the interface itself, so
// only the logical unit 0 can carry it. The CMDB value is emitted as-is, true
// or false; the leaf is only absent when the CMDB has no value.
func applyIPv6LinkLocalOnly(hostname string, logicalInterface *cmdbif.LogicalInterface, parent *openconfig.Interface) {
	if logicalInterface.UseIPv6LinkLocalOnly == nil {
		return
	}

	if *logicalInterface.Index != 0 {
		if *logicalInterface.UseIPv6LinkLocalOnly {
			log.Warn().Msgf("use_ipv6_link_local_only on logical interface %s of %s cannot be emitted: not unit 0", logicalInterface.Name(), hostname)
		}
		return
	}

	parent.UseIpv6LinkLocalOnly = logicalInterface.UseIPv6LinkLocalOnly
}

// applyMtu maps the CMDB logical interface MTU to OpenConfig. OpenConfig only
// models the MTU on the interface itself and on the IPv4/IPv6 containers of a
// subinterface, so the MTU of the logical unit 0 is set on the parent
// interface and the MTU of other units is set on their address families.
func applyMtu(hostname string, logicalInterface *cmdbif.LogicalInterface, parent *openconfig.Interface, subinterface *openconfig.Interface_Subinterface) error {
	if logicalInterface.Mtu == 0 {
		return nil
	}

	name := logicalInterface.Name()
	if logicalInterface.Mtu > math.MaxUint16 {
		return fmt.Errorf("invalid MTU %d on logical interface %s of %s", logicalInterface.Mtu, name, hostname)
	}
	mtu := uint16(logicalInterface.Mtu)

	if *logicalInterface.Index == 0 {
		parent.Mtu = &mtu
		return nil
	}

	if subinterface.Ipv4 == nil && subinterface.Ipv6 == nil {
		log.Warn().Msgf("MTU %d on logical interface %s of %s cannot be emitted: no address family and not unit 0", mtu, name, hostname)
		return nil
	}
	if subinterface.Ipv4 != nil {
		subinterface.Ipv4.Mtu = &mtu
	}
	if subinterface.Ipv6 != nil {
		ipv6Mtu := logicalInterface.Mtu
		subinterface.Ipv6.Mtu = &ipv6Mtu
	}
	return nil
}

// applySwitchedVlan attaches the 802.1Q attributes of a logical interface to
// its parent Ethernet or LAG interface.
func applySwitchedVlan(hostname string, logicalInterface *cmdbif.LogicalInterface, parent *openconfig.Interface) error {
	switchedVlan, err := switchedVlanToOpenconfig(hostname, logicalInterface)
	if err != nil {
		return err
	}
	if switchedVlan == nil {
		return nil
	}

	name := logicalInterface.Name()

	if parent.Type == openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd {
		if parent.Ethernet == nil {
			parent.Ethernet = &openconfig.Interface_Ethernet{}
		}
		if parent.Ethernet.SwitchedVlan != nil {
			return fmt.Errorf("conflicting 802.1Q configurations on device interface %s of %s", logicalInterface.ParentInterface.Name, hostname)
		}
		parent.Ethernet.SwitchedVlan = switchedVlan
		return nil
	}

	if parent.Type == openconfig.IETFInterfaces_InterfaceType_ieee8023adLag {
		if parent.Aggregation == nil {
			parent.Aggregation = &openconfig.Interface_Aggregation{}
		}
		if parent.Aggregation.SwitchedVlan != nil {
			return fmt.Errorf("conflicting 802.1Q configurations on device interface %s of %s", logicalInterface.ParentInterface.Name, hostname)
		}
		parent.Aggregation.SwitchedVlan = toAggregationSwitchedVlan(switchedVlan)
		return nil
	}

	return fmt.Errorf("802.1Q configuration on logical interface %s of %s is not supported: parent is neither an Ethernet nor a LAG interface", name, hostname)
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

// addNetworkInstanceBinding registers the logical interface in its network
// instance (VRF): explicit VRF first, default network instance for L3
// interfaces without a VRF, no binding otherwise.
// parentName is the OpenConfig name of the parent interface.
func addNetworkInstanceBinding(bindings NetworkInstanceInterfaces, logicalInterface *cmdbif.LogicalInterface, parentName string) {
	var networkInstance string
	switch {
	case logicalInterface.Vrf != nil:
		networkInstance = logicalInterface.Vrf.Name
	case logicalInterface.IPv4Address != nil || logicalInterface.IPv6Address != nil:
		networkInstance = DefaultNetworkInstance
	default:
		return
	}

	id := parentName + "." + strconv.FormatUint(uint64(*logicalInterface.Index), 10)
	if bindings[networkInstance] == nil {
		bindings[networkInstance] = make(map[string]*openconfig.NetworkInstance_Interface)
	}
	bindings[networkInstance][id] = &openconfig.NetworkInstance_Interface{
		Id:           &id,
		Interface:    &parentName,
		Subinterface: logicalInterface.Index,
	}
}
