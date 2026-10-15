package interfaces

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	cmdbif "github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/rs/zerolog/log"
)

// ManagementInterfaceName is the name of the management interface of a
// device in the NOS, the one SONiC configures through MGMT_PORT and
// MGMT_INTERFACE. It is matched against the OpenConfig interface name, i.e.
// after the port layout translation when the interface has a layout entry.
const ManagementInterfaceName = "eth0"

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

// portNumber extracts the first integer of a port short name (etp1, etp3a).
var portNumber = regexp.MustCompile(`[0-9]+`)

// PortIndex derives the front-panel port index from the port short name of
// the port layout: the first integer of the name, counted from zero (etp1 is
// index 0, etp3a is index 2). It returns false when the name carries no
// usable number.
func PortIndex(shortName string) (uint16, bool) {
	number, err := strconv.ParseUint(portNumber.FindString(shortName), 10, 16)
	if err != nil || number == 0 {
		return 0, false
	}
	return uint16(number - 1), true
}

// InterfaceName returns the name a CMDB device interface bears in OpenConfig:
// the vendor name of its port layout entry when the interface is a
// front-panel port (the CMDB often names those after their remote end, e.g.
// to_spine_01), its CMDB name otherwise.
func InterfaceName(deviceInterface *cmdbif.DeviceInterface, port *cmdbif.PortLayout) string {
	return PortName(deviceInterface.Name, port)
}

// PortName returns the name a CMDB interface bears on its device: the vendor
// name of its port layout entry when it has one, its CMDB name otherwise.
func PortName(cmdbName string, port *cmdbif.PortLayout) string {
	if port != nil && port.VendorName != "" {
		return port.VendorName
	}
	return cmdbName
}

// applyNeighbor sets the remote end of the link connected to the interface.
// OpenConfig path: /interfaces/interface/config/neighbor-{name,port}.
func applyNeighbor(iface *openconfig.Interface, neighbor *cmdbif.Neighbor) {
	if neighbor == nil {
		return
	}
	name := neighbor.Device
	port := PortName(neighbor.Interface, neighbor.Port)
	iface.NeighborName = &name
	iface.NeighborPort = &port
}

// DeviceInterfaceToOpenconfig converts one CMDB device (physical) interface to OpenConfig.
// OpenConfig path: /interfaces/interface/.
//
// port is the port layout entry of the interface, nil when the interface is
// not a front-panel port or when the device has no port layout. withPortIndex
// emits the front-panel port index derived from the port short name, which
// only AFK managed devices need (see PortIndex).
func DeviceInterfaceToOpenconfig(hostname string, deviceInterface *cmdbif.DeviceInterface, port *cmdbif.PortLayout, withPortIndex bool) (*openconfig.Interface, error) {
	name := InterfaceName(deviceInterface, port)

	iface := openconfig.Interface{
		Name:    &name,
		Enabled: deviceInterface.Enabled,
	}

	if deviceInterface.Description != "" {
		iface.Description = &deviceInterface.Description
	}

	// The management interface is identified by its NOS name, after the port
	// layout translation: OpenConfig has no type for it (it is ethernetCsmacd
	// like a front-panel port) and its vendor name (eth0) is a naming
	// convention of its own.
	if name == ManagementInterfaceName {
		management := true
		iface.Management = &management
	}

	ifaceType := interfaceTypeFromName(name)
	if ifaceType == openconfig.IETFInterfaces_InterfaceType_UNSET && (port != nil || deviceInterface.Speed != 0) {
		// A port layout entry is a physical front-panel port by definition,
		// and only physical ports carry a speed in the CMDB. This covers the
		// CMDB naming after the remote end (to_L3-SP01) when the device has
		// no port layout.
		ifaceType = openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd
	}
	if ifaceType == openconfig.IETFInterfaces_InterfaceType_UNSET {
		log.Warn().Msgf("unable to derive the type of interface %s on %s from its name, type is not emitted", name, hostname)
	} else {
		iface.Type = ifaceType
	}

	// Ethernet attributes only make sense on Ethernet interfaces. The CMDB
	// stores defaults (e.g. autonegotiation=true) on every interface kind,
	// so they are only emitted when the interface is an Ethernet one.
	if ifaceType == openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd {
		ethernet, err := ethernetToOpenconfig(hostname, deviceInterface, port, withPortIndex)
		if err != nil {
			return nil, err
		}
		iface.Ethernet = ethernet
	}

	return &iface, nil
}

// ethernetToOpenconfig converts the Ethernet attributes of a CMDB device
// interface and of its port layout entry (alias, lanes, index).
// OpenConfig path: /interfaces/interface/ethernet/.
// It returns nil when no Ethernet attribute is set in the CMDB.
func ethernetToOpenconfig(hostname string, deviceInterface *cmdbif.DeviceInterface, port *cmdbif.PortLayout, withPortIndex bool) (*openconfig.Interface_Ethernet, error) {
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
		if withPortIndex {
			if index, ok := PortIndex(port.VendorShortName); ok {
				ethernet.Index = &index
			} else {
				log.Debug().Msgf("no port index derivable from %q for interface %s of %s", port.VendorShortName, deviceInterface.Name, hostname)
			}
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
