package interfaces_test

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	ifconvertors "github.com/criteo/data-aggregation-api/internal/convertor/interfaces"
	cmdbif "github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/criteo/data-aggregation-api/internal/types"
	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/ygot/ygot"
)

func newManagementRoute(parent string, index uint32, kind string, prefix string, nextHop string) *cmdbif.ManagementRoute {
	ip, network, err := net.ParseCIDR(prefix)
	if err != nil {
		panic(err)
	}
	netmask, _ := network.Mask.Size()

	route := cmdbif.ManagementRoute{
		Kind:   kind,
		Prefix: types.CIDR{IP: ip, Netmask: netmask},
	}
	route.LogicalInterface.Index = ygot.Uint32(index)
	route.LogicalInterface.ParentInterface.Name = parent
	route.LogicalInterface.ParentInterface.Device.Name = hostname
	if nextHop != "" {
		gateway := net.ParseIP(nextHop)
		route.NextHop = &gateway
	}
	return &route
}

// sonicManagementRoutes are the routes of a SONiC device, as provisioned
// from a MGMT_INTERFACE entry such as
// "eth0|10.0.3.69/21": {"gwaddr": "10.0.0.1", "forced_mgmt_routes": [...]}.
func sonicManagementRoutes(gateway string) []*cmdbif.ManagementRoute {
	return []*cmdbif.ManagementRoute{
		newManagementRoute("eth0", 0, cmdbif.ManagementRouteStatic, "0.0.0.0/0", gateway),
		newManagementRoute("eth0", 0, cmdbif.ManagementRouteForced, "172.16.0.0/12", ""),
		newManagementRoute("eth0", 0, cmdbif.ManagementRouteForced, "192.168.0.0/16", ""),
	}
}

// managementDevice converts a device with an eth0 management interface
// addressed with address in the vrf (no VRF when empty). The management
// interface is not a front-panel port: it is not in the port layout.
func managementDevice(t *testing.T, address string, vrf string) (map[string]*openconfig.Interface, ifconvertors.NetworkInstanceInterfaces, []*cmdbif.DeviceInterface, cmdbif.PortLayoutTable) {
	t.Helper()

	mgmt := newDeviceInterface("eth0", true)
	ethernet0 := newDeviceInterface("Ethernet0", true)
	deviceInterfaces := []*cmdbif.DeviceInterface{mgmt, ethernet0}

	unit0 := newLogicalInterface("eth0", 0, cmdbif.TypeL3)
	unit0.IPv4Address = newIPAddress(address, 4)
	if vrf != "" {
		unit0.Vrf = &cmdbif.VRFLite{Name: vrf}
	}

	portLayout := cmdbif.PortLayoutTable{}

	interfaces, bindings, err := ifconvertors.InterfacesToOpenconfig(hostname, deviceInterfaces, []*cmdbif.LogicalInterface{unit0}, portLayout, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	return interfaces, bindings, deviceInterfaces, portLayout
}

func TestManagementFlag(t *testing.T) {
	interfaces, _, _, _ := managementDevice(t, "10.0.3.69/21", "prod")

	if interfaces["eth0"].Management == nil || !*interfaces["eth0"].Management {
		t.Error("eth0 is not marked as the management interface")
	}
	if interfaces["Ethernet0"].Management != nil {
		t.Error("Ethernet0 is marked as the management interface")
	}
}

func TestManagementRoutesToOpenconfig(t *testing.T) {
	tests := []struct {
		name            string
		address         string
		gateway         string
		vrf             string
		networkInstance string
	}{
		// "eth0|10.0.19.72/21", gwaddr 10.0.16.1
		{"management in prod VRF", "10.0.19.72/21", "10.0.16.1", "prod", "prod"},
		// "eth0|10.0.3.69/21", gwaddr 10.0.0.1
		{"management in default network instance", "10.0.3.69/21", "10.0.0.1", "", ifconvertors.DefaultNetworkInstance},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			interfaces, bindings, deviceInterfaces, portLayout := managementDevice(t, test.address, test.vrf)

			protocols, err := ifconvertors.ManagementRoutesToOpenconfig(hostname, sonicManagementRoutes(test.gateway), deviceInterfaces, portLayout, interfaces, bindings)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			// forced routes on the management subinterface, in CMDB order
			wantForced := []string{"172.16.0.0/12", "192.168.0.0/16"}
			if diff := cmp.Diff(interfaces["eth0"].Subinterface[0].Ipv4.ForcedRoutes, wantForced); diff != "" {
				t.Errorf("unexpected forced routes: %s", diff)
			}
			if interfaces["Ethernet0"].Subinterface != nil {
				t.Error("Ethernet0 got subinterfaces")
			}

			// the default route in the network instance of the management subinterface
			wantProtocols := ifconvertors.NetworkInstanceProtocols{
				test.networkInstance: {
					ifconvertors.StaticProtocolKey: {
						Identifier: openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
						Name:       ygot.String("static"),
						Static: map[string]*openconfig.NetworkInstance_Protocol_Static{
							"0.0.0.0/0": {
								Prefix: ygot.String("0.0.0.0/0"),
								NextHop: map[string]*openconfig.NetworkInstance_Protocol_Static_NextHop{
									"0": {
										Index:   ygot.String("0"),
										NextHop: openconfig.UnionString(test.gateway),
										InterfaceRef: &openconfig.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
											Interface:    ygot.String("eth0"),
											Subinterface: ygot.Uint32(0),
										},
									},
								},
							},
						},
					},
				},
			}
			if diff := cmp.Diff(protocols, wantProtocols); diff != "" {
				t.Errorf("unexpected protocols: %s", diff)
			}
		})
	}
}

func TestManagementRoutesToOpenconfigNoRoutes(t *testing.T) {
	// a device without management routes (not SONiC) is the normal case
	interfaces, bindings, deviceInterfaces, portLayout := managementDevice(t, "10.0.3.69/21", "prod")

	protocols, err := ifconvertors.ManagementRoutesToOpenconfig(hostname, nil, deviceInterfaces, portLayout, interfaces, bindings)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(protocols) != 0 {
		t.Errorf("unexpected protocols: %v", protocols)
	}
	if interfaces["eth0"].Subinterface[0].Ipv4.ForcedRoutes != nil {
		t.Errorf("unexpected forced routes: %v", interfaces["eth0"].Subinterface[0].Ipv4.ForcedRoutes)
	}
}

func TestManagementRoutesToOpenconfigDuplicateForcedRoute(t *testing.T) {
	interfaces, bindings, deviceInterfaces, portLayout := managementDevice(t, "10.0.3.69/21", "prod")
	routes := append(sonicManagementRoutes("10.0.0.1"), newManagementRoute("eth0", 0, cmdbif.ManagementRouteForced, "172.16.0.0/12", ""))

	if _, err := ifconvertors.ManagementRoutesToOpenconfig(hostname, routes, deviceInterfaces, portLayout, interfaces, bindings); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	wantForced := []string{"172.16.0.0/12", "192.168.0.0/16"}
	if diff := cmp.Diff(interfaces["eth0"].Subinterface[0].Ipv4.ForcedRoutes, wantForced); diff != "" {
		t.Errorf("unexpected forced routes: %s", diff)
	}
}

func TestManagementRoutesToOpenconfigForcedWithoutStatic(t *testing.T) {
	// accepted with a warning: SONiC accepts it
	interfaces, bindings, deviceInterfaces, portLayout := managementDevice(t, "10.0.3.69/21", "prod")
	routes := []*cmdbif.ManagementRoute{newManagementRoute("eth0", 0, cmdbif.ManagementRouteForced, "172.16.0.0/12", "")}

	protocols, err := ifconvertors.ManagementRoutesToOpenconfig(hostname, routes, deviceInterfaces, portLayout, interfaces, bindings)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(protocols) != 0 {
		t.Errorf("unexpected protocols: %v", protocols)
	}
}

func TestManagementRoutesToOpenconfigErrors(t *testing.T) {
	tests := []struct {
		name   string
		routes []*cmdbif.ManagementRoute
		want   string
	}{
		{
			"not the management interface",
			[]*cmdbif.ManagementRoute{newManagementRoute("Ethernet0", 0, cmdbif.ManagementRouteStatic, "0.0.0.0/0", "10.0.0.1")},
			"not the management interface",
		},
		{
			"unknown device interface",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth1", 0, cmdbif.ManagementRouteStatic, "0.0.0.0/0", "10.0.0.1")},
			"unknown device interface eth1",
		},
		{
			"unknown logical interface",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth0", 1, cmdbif.ManagementRouteStatic, "0.0.0.0/0", "10.0.0.1")},
			"unknown logical interface eth0.1",
		},
		{
			"off-subnet next-hop",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth0", 0, cmdbif.ManagementRouteStatic, "0.0.0.0/0", "10.0.16.1")},
			"not in the subnet",
		},
		{
			"next-hop is the interface address",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth0", 0, cmdbif.ManagementRouteStatic, "0.0.0.0/0", "10.0.3.69")},
			"is the address of the interface",
		},
		{
			"static route without next-hop",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth0", 0, cmdbif.ManagementRouteStatic, "0.0.0.0/0", "")},
			"has no next-hop",
		},
		{
			"forced route with a next-hop",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth0", 0, cmdbif.ManagementRouteForced, "172.16.0.0/12", "10.0.0.1")},
			"forced routes resolve through the static route",
		},
		{
			"IPv6 static route",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth0", 0, cmdbif.ManagementRouteStatic, "::/0", "2001:db8::1")},
			"IPv6 management routes are not supported",
		},
		{
			"IPv6 forced route",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth0", 0, cmdbif.ManagementRouteForced, "2001:db8::/32", "")},
			"IPv6 management routes are not supported",
		},
		{
			"IPv6 next-hop",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth0", 0, cmdbif.ManagementRouteStatic, "0.0.0.0/0", "2001:db8::1")},
			"is not an IPv4 address",
		},
		{
			"duplicate static route",
			[]*cmdbif.ManagementRoute{
				newManagementRoute("eth0", 0, cmdbif.ManagementRouteStatic, "0.0.0.0/0", "10.0.0.1"),
				newManagementRoute("eth0", 0, cmdbif.ManagementRouteStatic, "0.0.0.0/0", "10.0.0.2"),
			},
			"duplicate static route 0.0.0.0/0",
		},
		{
			"unknown kind",
			[]*cmdbif.ManagementRoute{newManagementRoute("eth0", 0, "connected", "0.0.0.0/0", "10.0.0.1")},
			"unsupported management route kind",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			interfaces, bindings, deviceInterfaces, portLayout := managementDevice(t, "10.0.3.69/21", "prod")

			_, err := ifconvertors.ManagementRoutesToOpenconfig(hostname, test.routes, deviceInterfaces, portLayout, interfaces, bindings)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}

func TestManagementRoutesToOpenconfigNoIPv4Address(t *testing.T) {
	mgmt := newDeviceInterface("eth0", true)
	unit0 := newLogicalInterface("eth0", 0, cmdbif.TypeL3)
	unit0.Vrf = &cmdbif.VRFLite{Name: "prod"}
	portLayout := cmdbif.PortLayoutTable{}

	interfaces, bindings, err := ifconvertors.InterfacesToOpenconfig(hostname, []*cmdbif.DeviceInterface{mgmt}, []*cmdbif.LogicalInterface{unit0}, portLayout, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	_, err = ifconvertors.ManagementRoutesToOpenconfig(hostname, sonicManagementRoutes("10.0.0.1"), []*cmdbif.DeviceInterface{mgmt}, portLayout, interfaces, bindings)
	if err == nil || !strings.Contains(err.Error(), "no IPv4 address on eth0.0") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestManagementRoutesToOpenconfigValidDocument emits a full OpenConfig
// document through ygot with validation enabled (leafrefs included) and
// checks the RFC7951 output.
func TestManagementRoutesToOpenconfigValidDocument(t *testing.T) {
	interfaces, bindings, deviceInterfaces, portLayout := managementDevice(t, "10.0.19.72/21", "prod")

	protocols, err := ifconvertors.ManagementRoutesToOpenconfig(hostname, sonicManagementRoutes("10.0.16.1"), deviceInterfaces, portLayout, interfaces, bindings)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	device := openconfig.Device{
		Interface:       interfaces,
		NetworkInstance: map[string]*openconfig.NetworkInstance{},
	}
	for networkInstanceName, interfaceBindings := range bindings {
		name := networkInstanceName
		device.NetworkInstance[networkInstanceName] = &openconfig.NetworkInstance{
			Name:      &name,
			Type:      openconfig.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
			Interface: interfaceBindings,
			Protocol:  protocols[networkInstanceName],
		}
	}

	out, err := ygot.EmitJSON(&device, &ygot.EmitJSONConfig{Format: ygot.RFC7951, SkipValidation: false})
	if err != nil {
		t.Fatalf("failed to emit JSON: %s", err)
	}

	var document map[string]any
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		t.Fatalf("invalid JSON: %s", err)
	}

	for _, want := range []string{
		`"management": true`,
		`"forced-routes": [`,
		`"172.16.0.0/12"`,
		`"192.168.0.0/16"`,
		`"identifier": "STATIC"`,
		`"name": "static"`,
		`"prefix": "0.0.0.0/0"`,
		`"next-hop": "10.0.16.1"`,
		`"interface": "eth0"`,
		`"subinterface": 0`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in the emitted JSON:\n%s", want, out)
		}
	}
}
