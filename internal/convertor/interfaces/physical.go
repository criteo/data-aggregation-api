package interfaces

import (
	"fmt"
	"strings"

	cmdbif "github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/rs/zerolog/log"
)

// speedToOpenconfig maps the CMDB interface speed (in Mb/s) to the OpenConfig
// Ethernet speed identities.
var speedToOpenconfig = map[uint32]openconfig.E_IfEthernet_ETHERNET_SPEED{
	10:      openconfig.IfEthernet_ETHERNET_SPEED_SPEED_10MB,
	100:     openconfig.IfEthernet_ETHERNET_SPEED_SPEED_100MB,
	1000:    openconfig.IfEthernet_ETHERNET_SPEED_SPEED_1GB,
	2500:    openconfig.IfEthernet_ETHERNET_SPEED_SPEED_2500MB,
	5000:    openconfig.IfEthernet_ETHERNET_SPEED_SPEED_5GB,
	10000:   openconfig.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	25000:   openconfig.IfEthernet_ETHERNET_SPEED_SPEED_25GB,
	40000:   openconfig.IfEthernet_ETHERNET_SPEED_SPEED_40GB,
	50000:   openconfig.IfEthernet_ETHERNET_SPEED_SPEED_50GB,
	100000:  openconfig.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	200000:  openconfig.IfEthernet_ETHERNET_SPEED_SPEED_200GB,
	400000:  openconfig.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
	800000:  openconfig.IfEthernet_ETHERNET_SPEED_SPEED_800GB,
	1600000: openconfig.IfEthernet_ETHERNET_SPEED_SPEED_1600GB,
}

// fecToOpenconfig maps the CMDB FEC value to the OpenConfig fec-mode
// identities. auto and rs have no OpenConfig identity (OpenConfig only names
// clause-specific Reed-Solomon modes), so they use the afk-interfaces-ext ones.
var fecToOpenconfig = map[string]openconfig.E_IfEthernet_INTERFACE_FEC{
	"auto": openconfig.IfEthernet_INTERFACE_FEC_FEC_AUTO,
	"rs":   openconfig.IfEthernet_INTERFACE_FEC_FEC_RS,
	"fc":   openconfig.IfEthernet_INTERFACE_FEC_FEC_FC,
}

// interfaceTypeFromName derives the OpenConfig interface type from the
// interface naming conventions used in the CMDB. It returns UNSET when the
// name does not match any known convention: the type is then simply not
// emitted rather than being invented.
func interfaceTypeFromName(name string) openconfig.E_IETFInterfaces_InterfaceType {
	lowered := strings.ToLower(name)

	// evaluation order matters: "ethernet" must be tested before "eth"
	switch {
	case strings.HasPrefix(lowered, "loopback") || strings.HasPrefix(lowered, "lo"):
		return openconfig.IETFInterfaces_InterfaceType_softwareLoopback
	case strings.HasPrefix(lowered, "portchannel") || strings.HasPrefix(lowered, "port-channel") || strings.HasPrefix(lowered, "ae"):
		return openconfig.IETFInterfaces_InterfaceType_ieee8023adLag
	case strings.HasPrefix(lowered, "vlan") || strings.HasPrefix(lowered, "irb"):
		return openconfig.IETFInterfaces_InterfaceType_l3ipvlan
	case strings.HasPrefix(lowered, "mgmt") || strings.HasPrefix(lowered, "management"):
		return openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd
	case strings.HasPrefix(lowered, "ethernet") || strings.HasPrefix(lowered, "etp") ||
		strings.HasPrefix(lowered, "eth") || strings.HasPrefix(lowered, "swp") ||
		strings.HasPrefix(lowered, "et-") || strings.HasPrefix(lowered, "xe-") ||
		strings.HasPrefix(lowered, "ge-"):
		return openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd
	default:
		return openconfig.IETFInterfaces_InterfaceType_UNSET
	}
}

// interfaceType returns the OpenConfig type of a device interface, UNSET when
// it cannot be derived.
func interfaceType(hostname string, iface *cmdbif.Interface) openconfig.E_IETFInterfaces_InterfaceType {
	ifaceType := interfaceTypeFromName(iface.Name)
	if ifaceType == openconfig.IETFInterfaces_InterfaceType_UNSET && (iface.Port != nil || iface.DeviceInterface.Speed != 0) {
		// A port layout entry is a physical front-panel port by definition,
		// and only physical ports carry a speed in the CMDB. This covers the
		// CMDB naming after the remote end (to_L3-SP01) when the device has
		// no port layout.
		ifaceType = openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd
	}
	if ifaceType == openconfig.IETFInterfaces_InterfaceType_UNSET {
		log.Warn().Msgf("unable to derive the type of interface %s on %s from its name, type is not emitted", iface.Name, hostname)
	}
	return ifaceType
}

// DeviceInterfaceToOpenconfig converts one precomputed CMDB device interface,
// with its logical interfaces, to OpenConfig.
// OpenConfig path: /interfaces/interface/.
func DeviceInterfaceToOpenconfig(hostname string, iface *cmdbif.Interface) (*openconfig.Interface, error) {
	deviceInterface := iface.DeviceInterface
	name := iface.Name
	ifaceType := interfaceType(hostname, iface)

	// Ethernet attributes only make sense on Ethernet interfaces. The CMDB
	// stores defaults (e.g. autonegotiation=true) on every interface kind,
	// so they are only emitted when the interface is an Ethernet one.
	var ethernet *openconfig.Interface_Ethernet
	if ifaceType == openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd {
		var err error
		ethernet, err = ethernetToOpenconfig(hostname, deviceInterface, iface.Port)
		if err != nil {
			return nil, err
		}
	}

	subinterfaces, err := subinterfacesToOpenconfig(hostname, iface, ifaceType == openconfig.IETFInterfaces_InterfaceType_softwareLoopback)
	if err != nil {
		return nil, err
	}

	mtu, err := interfaceMtu(hostname, iface)
	if err != nil {
		return nil, err
	}

	switchedVlan, err := interfaceSwitchedVlan(hostname, iface)
	if err != nil {
		return nil, err
	}
	var aggregation *openconfig.Interface_Aggregation
	if switchedVlan != nil {
		switch ifaceType { //nolint:exhaustive // only Ethernet and LAG interfaces carry 802.1Q
		case openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd:
			if ethernet == nil {
				ethernet = &openconfig.Interface_Ethernet{}
			}
			ethernet.SwitchedVlan = switchedVlan
		case openconfig.IETFInterfaces_InterfaceType_ieee8023adLag:
			aggregation = &openconfig.Interface_Aggregation{SwitchedVlan: toAggregationSwitchedVlan(switchedVlan)}
		default:
			return nil, fmt.Errorf("802.1Q configuration on device interface %s of %s is not supported: it is neither an Ethernet nor a LAG interface", deviceInterface.Name, hostname)
		}
	}

	converted := openconfig.Interface{
		Name:                 &name,
		Enabled:              deviceInterface.Enabled,
		Type:                 ifaceType,
		Mtu:                  mtu,
		UseIpv6LinkLocalOnly: ipv6LinkLocalOnly(iface),
		Ethernet:             ethernet,
		Aggregation:          aggregation,
		Subinterface:         subinterfaces,
	}

	if deviceInterface.Description != "" {
		converted.Description = &deviceInterface.Description
	}

	// The management interface is identified by its NOS name, after the port
	// layout translation: OpenConfig has no type for it (it is ethernetCsmacd
	// like a front-panel port) and its vendor name (eth0) is a naming
	// convention of its own.
	if name == cmdbif.ManagementInterfaceName {
		management := true
		converted.Management = &management
	}

	// OpenConfig path: /interfaces/interface/config/neighbor-{name,port}.
	if iface.Neighbor != nil {
		neighborName, neighborPort := iface.Neighbor.Device, iface.Neighbor.Port
		converted.NeighborName = &neighborName
		converted.NeighborPort = &neighborPort
	}

	return &converted, nil
}

// ethernetToOpenconfig converts the Ethernet attributes of a CMDB device
// interface and of its port layout entry (alias, lanes, index).
// OpenConfig path: /interfaces/interface/ethernet/.
// It returns nil when no Ethernet attribute is set in the CMDB.
func ethernetToOpenconfig(hostname string, deviceInterface *cmdbif.DeviceInterface, port *cmdbif.PortLayout) (*openconfig.Interface_Ethernet, error) {
	ethernet := openconfig.Interface_Ethernet{
		AutoNegotiate: deviceInterface.Autonegotiation,
	}

	if port != nil {
		if port.VendorShortName != "" {
			ethernet.Alias = &port.VendorShortName
		}
		if lanes := port.LanesString(); lanes != "" {
			ethernet.Lanes = &lanes
		}
		if port.Index != nil {
			index := *port.Index
			ethernet.Index = &index
		} else {
			log.Debug().Msgf("no port index in the port layout of interface %s of %s", deviceInterface.Name, hostname)
		}
	}

	if deviceInterface.Speed != 0 {
		speed, ok := speedToOpenconfig[deviceInterface.Speed]
		if !ok {
			return nil, fmt.Errorf("unsupported speed %d Mb/s on interface %s of %s", deviceInterface.Speed, deviceInterface.Name, hostname)
		}
		ethernet.PortSpeed = speed
	}

	if deviceInterface.Fec != "" {
		fec, ok := fecToOpenconfig[deviceInterface.Fec]
		if !ok {
			return nil, fmt.Errorf("unsupported FEC %q on interface %s of %s", deviceInterface.Fec, deviceInterface.Name, hostname)
		}
		ethernet.FecMode = fec
	}

	if ethernet.AutoNegotiate == nil && ethernet.PortSpeed == openconfig.IfEthernet_ETHERNET_SPEED_UNSET &&
		ethernet.FecMode == openconfig.IfEthernet_INTERFACE_FEC_UNSET && ethernet.Alias == nil && ethernet.Lanes == nil && ethernet.Index == nil {
		return nil, nil //nolint:nilnil // no Ethernet attribute in the CMDB: nothing to emit
	}

	return &ethernet, nil
}
