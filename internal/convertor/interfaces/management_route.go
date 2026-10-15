package interfaces

import (
	"fmt"
	"net"
	"strconv"

	cmdbif "github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/rs/zerolog/log"
)

// StaticProtocolName is the name of the static routing protocol instance.
// OpenConfig path: /network-instances/network-instance/protocols/protocol[STATIC][static].
const StaticProtocolName = "static"

// StaticProtocolKey is the key of the static routing protocol instance of a
// network instance.
var StaticProtocolKey = openconfig.NetworkInstance_Protocol_Key{
	Identifier: openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
	Name:       StaticProtocolName,
}

// NetworkInstanceProtocols maps a network instance name to its routing
// protocol instances.
// OpenConfig path: /network-instances/network-instance/protocols/protocol/.
type NetworkInstanceProtocols map[string]map[openconfig.NetworkInstance_Protocol_Key]*openconfig.NetworkInstance_Protocol

// isManagement tells whether an interface is marked as the management interface.
func isManagement(iface *openconfig.Interface) bool {
	return iface.Management != nil && *iface.Management
}

// managementSubinterface is a management subinterface carrying management routes.
type managementSubinterface struct {
	name         string // display name, for messages
	parentName   string // OpenConfig name of the parent interface
	index        uint32
	subinterface *openconfig.Interface_Subinterface
	hasStatic    bool
	hasForced    bool
}

// ManagementRoutesToOpenconfig converts the CMDB management routes of one
// device to OpenConfig, on top of its converted interfaces:
//   - forced routes are emitted on the IPv4 container of their subinterface
//     (afk-interfaces-ext forced-routes), in CMDB order;
//   - static routes are emitted as static routes of the network instance the
//     subinterface is bound to, with the subinterface as interface-ref.
//
// The consumer contract for the management default route is "the 0.0.0.0/0
// static route whose interface-ref is the management interface".
//
// interfaces and bindings are the output of InterfacesToOpenconfig for the
// same device interfaces and port layout; interfaces is modified in place.
// It returns the routing protocol instances per network instance.
func ManagementRoutesToOpenconfig(hostname string, managementRoutes []*cmdbif.ManagementRoute, deviceInterfaces []*cmdbif.DeviceInterface, portLayout cmdbif.PortLayoutTable, interfaces map[string]*openconfig.Interface, bindings NetworkInstanceInterfaces) (NetworkInstanceProtocols, error) {
	protocols := make(NetworkInstanceProtocols)
	if len(managementRoutes) == 0 {
		return protocols, nil
	}

	names := make(map[string]string, len(deviceInterfaces))
	for _, deviceInterface := range deviceInterfaces {
		names[deviceInterface.Name] = InterfaceName(deviceInterface, portLayout[deviceInterface.Name])
	}

	subinterfaces := make(map[string]*managementSubinterface)
	// insertion order, to keep messages and processing deterministic
	var order []string

	for _, route := range managementRoutes {
		name := route.InterfaceName() + "." + strconv.FormatUint(uint64(route.SubinterfaceIndex()), 10)

		target, ok := subinterfaces[name]
		if !ok {
			var err error
			target, err = resolveManagementSubinterface(hostname, route, names, interfaces)
			if err != nil {
				return nil, err
			}
			subinterfaces[name] = target
			order = append(order, name)
		}

		switch route.Kind {
		case cmdbif.ManagementRouteForced:
			if err := applyForcedRoute(hostname, route, target); err != nil {
				return nil, err
			}
		case cmdbif.ManagementRouteStatic:
			if err := applyStaticRoute(hostname, route, target, bindings, protocols); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported management route kind %q for %s on %s", route.Kind, route.Prefix.String(), hostname)
		}
	}

	for _, name := range order {
		target := subinterfaces[name]
		if target.hasForced && !target.hasStatic {
			log.Warn().Msgf("forced routes on %s of %s without a static route: they resolve through nothing", name, hostname)
		}
	}

	return protocols, nil
}

// resolveManagementSubinterface finds the converted subinterface a management
// route is attached to, which must belong to the management interface and
// carry an IPv4 address.
func resolveManagementSubinterface(hostname string, route *cmdbif.ManagementRoute, names map[string]string, interfaces map[string]*openconfig.Interface) (*managementSubinterface, error) {
	cmdbName := route.InterfaceName()
	index := route.SubinterfaceIndex()
	name := cmdbName + "." + strconv.FormatUint(uint64(index), 10)

	parentName, ok := names[cmdbName]
	if !ok {
		return nil, fmt.Errorf("management route %s on %s references unknown device interface %s", route.Prefix.String(), hostname, cmdbName)
	}
	parent, ok := interfaces[parentName]
	if !ok {
		return nil, fmt.Errorf("management route %s on %s references unknown interface %s", route.Prefix.String(), hostname, parentName)
	}
	if !isManagement(parent) {
		return nil, fmt.Errorf("management route %s on %s is attached to %s, which is not the management interface", route.Prefix.String(), hostname, name)
	}

	subinterface, ok := parent.Subinterface[index]
	if !ok {
		return nil, fmt.Errorf("management route %s on %s references unknown logical interface %s", route.Prefix.String(), hostname, name)
	}
	if subinterface.Ipv4 == nil || len(subinterface.Ipv4.Address) == 0 {
		return nil, fmt.Errorf("management route %s on %s: no IPv4 address on %s", route.Prefix.String(), hostname, name)
	}

	return &managementSubinterface{
		name:         name,
		parentName:   parentName,
		index:        index,
		subinterface: subinterface,
	}, nil
}

// ipv4Prefix returns the route prefix as a string, or an error when it is not IPv4.
func ipv4Prefix(hostname string, route *cmdbif.ManagementRoute) (string, error) {
	if route.Prefix.IP.To4() == nil || route.Prefix.Netmask < 0 || route.Prefix.Netmask > 32 {
		return "", fmt.Errorf("management route %s on %s: IPv6 management routes are not supported", route.Prefix.String(), hostname)
	}
	return route.Prefix.String(), nil
}

// applyForcedRoute appends the prefix of a forced route to the forced routes
// of its subinterface, keeping the CMDB order and skipping duplicates.
// OpenConfig path: /interfaces/interface/subinterfaces/subinterface/ipv4/config/forced-routes.
func applyForcedRoute(hostname string, route *cmdbif.ManagementRoute, target *managementSubinterface) error {
	prefix, err := ipv4Prefix(hostname, route)
	if err != nil {
		return err
	}
	if route.NextHop != nil {
		return fmt.Errorf("forced route %s on %s of %s has a next-hop: forced routes resolve through the static route", prefix, target.name, hostname)
	}

	for _, existing := range target.subinterface.Ipv4.ForcedRoutes {
		if existing == prefix {
			log.Warn().Msgf("duplicate forced route %s on %s of %s", prefix, target.name, hostname)
			return nil
		}
	}

	target.subinterface.Ipv4.ForcedRoutes = append(target.subinterface.Ipv4.ForcedRoutes, prefix)
	target.hasForced = true
	return nil
}

// applyStaticRoute emits a static route in the network instance its
// subinterface is bound to, with the subinterface as interface-ref. The
// next-hop must be an IPv4 address of the subnet of the subinterface, other
// than its own address.
// OpenConfig path: /network-instances/network-instance/protocols/protocol/static-routes/static/.
func applyStaticRoute(hostname string, route *cmdbif.ManagementRoute, target *managementSubinterface, bindings NetworkInstanceInterfaces, protocols NetworkInstanceProtocols) error {
	prefix, err := ipv4Prefix(hostname, route)
	if err != nil {
		return err
	}

	if route.NextHop == nil {
		return fmt.Errorf("static route %s on %s of %s has no next-hop", prefix, target.name, hostname)
	}
	nextHop := route.NextHop.To4()
	if nextHop == nil {
		return fmt.Errorf("static route %s on %s of %s: next-hop %s is not an IPv4 address", prefix, target.name, hostname, route.NextHop.String())
	}
	if err := checkNextHopInSubnet(hostname, prefix, nextHop, target); err != nil {
		return err
	}

	networkInstance, err := boundNetworkInstance(hostname, target, bindings)
	if err != nil {
		return err
	}

	if protocols[networkInstance] == nil {
		protocols[networkInstance] = make(map[openconfig.NetworkInstance_Protocol_Key]*openconfig.NetworkInstance_Protocol)
	}
	protocol, ok := protocols[networkInstance][StaticProtocolKey]
	if !ok {
		name := StaticProtocolName
		protocol = &openconfig.NetworkInstance_Protocol{
			Identifier: openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
			Name:       &name,
			Static:     make(map[string]*openconfig.NetworkInstance_Protocol_Static),
		}
		protocols[networkInstance][StaticProtocolKey] = protocol
	}

	if _, duplicate := protocol.Static[prefix]; duplicate {
		return fmt.Errorf("duplicate static route %s in network instance %s of %s", prefix, networkInstance, hostname)
	}

	// A single next-hop per static route: the management gateway.
	nextHopIndex := "0"
	nextHopAddress := nextHop.String()
	parentName := target.parentName
	index := target.index
	staticPrefix := prefix

	protocol.Static[prefix] = &openconfig.NetworkInstance_Protocol_Static{
		Prefix: &staticPrefix,
		NextHop: map[string]*openconfig.NetworkInstance_Protocol_Static_NextHop{
			nextHopIndex: {
				Index:   &nextHopIndex,
				NextHop: openconfig.UnionString(nextHopAddress),
				InterfaceRef: &openconfig.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface:    &parentName,
					Subinterface: &index,
				},
			},
		},
	}
	target.hasStatic = true
	return nil
}

// checkNextHopInSubnet verifies the next-hop is reachable on the
// subinterface: in the subnet of one of its IPv4 addresses, and not one of
// those addresses.
func checkNextHopInSubnet(hostname string, prefix string, nextHop net.IP, target *managementSubinterface) error {
	for _, address := range target.subinterface.Ipv4.Address {
		if address.Ip == nil || address.PrefixLength == nil {
			continue
		}
		ip := net.ParseIP(*address.Ip)
		if ip == nil || ip.To4() == nil {
			continue
		}
		if ip.Equal(nextHop) {
			return fmt.Errorf("static route %s on %s of %s: next-hop %s is the address of the interface", prefix, target.name, hostname, nextHop.String())
		}
		subnet := net.IPNet{IP: ip.To4(), Mask: net.CIDRMask(int(*address.PrefixLength), 32)}
		if subnet.Contains(nextHop) {
			return nil
		}
	}

	return fmt.Errorf("static route %s on %s of %s: next-hop %s is not in the subnet of the interface", prefix, target.name, hostname, nextHop.String())
}

// boundNetworkInstance returns the name of the network instance a
// subinterface is bound to.
func boundNetworkInstance(hostname string, target *managementSubinterface, bindings NetworkInstanceInterfaces) (string, error) {
	id := target.parentName + "." + strconv.FormatUint(uint64(target.index), 10)
	for networkInstance, interfaceBindings := range bindings {
		if _, ok := interfaceBindings[id]; ok {
			return networkInstance, nil
		}
	}
	return "", fmt.Errorf("static route on %s of %s: the interface is bound to no network instance", target.name, hostname)
}
