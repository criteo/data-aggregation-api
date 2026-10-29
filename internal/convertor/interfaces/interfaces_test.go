package interfaces_test

import (
	"net"
	"strings"
	"testing"

	ifconvertors "github.com/criteo/data-aggregation-api/internal/convertor/interfaces"
	"github.com/criteo/data-aggregation-api/internal/ingestor/cmdb"
	cmdbif "github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/criteo/data-aggregation-api/internal/types"
	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/ygot/ygot"
)

const hostname = "tor01-01"

func newDeviceInterface(name string, enabled bool) *cmdbif.DeviceInterface {
	deviceInterface := cmdbif.DeviceInterface{
		Name:    name,
		Enabled: ygot.Bool(enabled),
	}
	deviceInterface.Device.Name = hostname
	return &deviceInterface
}

func newLogicalInterface(parent string, index uint32, ifaceType string) *cmdbif.LogicalInterface {
	logicalInterface := cmdbif.LogicalInterface{
		Index:   ygot.Uint32(index),
		Enabled: ygot.Bool(true),
		Type:    ifaceType,
	}
	logicalInterface.ParentInterface.Name = parent
	logicalInterface.ParentInterface.Device.Name = hostname
	return &logicalInterface
}

func newIPAddress(address string, family int) *cmdbif.IPAddress {
	ip, network, err := net.ParseCIDR(address)
	if err != nil {
		panic(err)
	}
	netmask, _ := network.Mask.Size()
	return &cmdbif.IPAddress{
		Address: types.CIDR{IP: ip, Netmask: netmask},
		Family:  family,
	}
}

// toOpenconfig precomputes then converts the interfaces of one device, as
// NewDevice and Generateconfigs do.
func toOpenconfig(t *testing.T, deviceInterfaces []*cmdbif.DeviceInterface, logicalInterfaces []*cmdbif.LogicalInterface,
	portLayout cmdbif.PortLayoutTable, neighbors cmdbif.NeighborTable,
) (map[string]*openconfig.Interface, ifconvertors.NetworkInstanceInterfaces, error) {
	t.Helper()
	ifaces, err := cmdb.ResolveInterfaces(hostname, deviceInterfaces, logicalInterfaces, nil, portLayout, neighbors)
	if err != nil {
		return nil, nil, err
	}
	return ifconvertors.InterfacesToOpenconfig(hostname, ifaces)
}

func TestInterfacesToOpenconfig(t *testing.T) {
	// physical interfaces
	ethernet0 := newDeviceInterface("Ethernet0", true)
	ethernet0.Autonegotiation = ygot.Bool(true)
	ethernet0.Speed = 100000
	ethernet0.Fec = "rs"
	ethernet0.Description = "to spine01-01"

	ethernet8 := newDeviceInterface("Ethernet8", false) // no logical interface, no Ethernet attributes

	svi := newDeviceInterface("vlan800", true)
	svi.Autonegotiation = ygot.Bool(true) // CMDB default, must not be emitted on an SVI

	loopback := newDeviceInterface("Loopback0", true)

	console := newDeviceInterface("console", true) // unknown type, not emitted

	// logical interfaces
	ethernet0Unit0 := newLogicalInterface("Ethernet0", 0, cmdbif.TypeL3)
	ethernet0Unit0.Mtu = 9216
	ethernet0Unit0.Vrf = &cmdbif.VRFLite{Name: "prod"}
	ethernet0Unit0.IPv4Address = newIPAddress("100.64.0.11/31", 4)
	ethernet0Unit0.IPv6Address = newIPAddress("2001:db8::11/127", 6)
	ethernet0Unit0.UseIPv6LinkLocalOnly = ygot.Bool(false) // exposed as-is
	ethernet0Unit0.Description = "uplink"

	sviUnit0 := newLogicalInterface("vlan800", 0, cmdbif.TypeL3)
	sviUnit0.IPv4Address = newIPAddress("10.1.4.54/21", 4)

	loopbackUnit0 := newLogicalInterface("Loopback0", 0, cmdbif.TypeL3)
	loopbackUnit0.IPv4Address = newIPAddress("10.2.180.112/21", 4)    // pool mask in the CMDB, exposed as /32
	loopbackUnit0.IPv6Address = newIPAddress("2001:db8:1::112/64", 6) // exposed as /128

	consoleUnit0 := newLogicalInterface("console", 0, cmdbif.TypeL1)

	got, gotBindings, err := toOpenconfig(t,
		[]*cmdbif.DeviceInterface{ethernet0, ethernet8, svi, loopback, console},
		[]*cmdbif.LogicalInterface{ethernet0Unit0, sviUnit0, loopbackUnit0, consoleUnit0},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	want := map[string]*openconfig.Interface{
		"Ethernet0": {
			Name:                 ygot.String("Ethernet0"),
			Enabled:              ygot.Bool(true),
			Type:                 openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Description:          ygot.String("to spine01-01"),
			Mtu:                  ygot.Uint16(9216),
			UseIpv6LinkLocalOnly: ygot.Bool(false),
			Ethernet: &openconfig.Interface_Ethernet{
				AutoNegotiate: ygot.Bool(true),
				PortSpeed:     openconfig.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
				FecMode:       openconfig.IfEthernet_INTERFACE_FEC_FEC_RS,
			},
			Subinterface: map[uint32]*openconfig.Interface_Subinterface{
				0: {
					Index:       ygot.Uint32(0),
					Enabled:     ygot.Bool(true),
					Description: ygot.String("uplink"),
					Ipv4: &openconfig.Interface_Subinterface_Ipv4{
						Address: map[string]*openconfig.Interface_Subinterface_Ipv4_Address{
							"100.64.0.11": {Ip: ygot.String("100.64.0.11"), PrefixLength: ygot.Uint8(31)},
						},
					},
					Ipv6: &openconfig.Interface_Subinterface_Ipv6{
						Address: map[string]*openconfig.Interface_Subinterface_Ipv6_Address{
							"2001:db8::11": {Ip: ygot.String("2001:db8::11"), PrefixLength: ygot.Uint8(127)},
						},
					},
				},
			},
		},
		"Ethernet8": {
			Name:    ygot.String("Ethernet8"),
			Enabled: ygot.Bool(false),
			Type:    openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd,
		},
		"vlan800": {
			Name:    ygot.String("vlan800"),
			Enabled: ygot.Bool(true),
			Type:    openconfig.IETFInterfaces_InterfaceType_l3ipvlan,
			Subinterface: map[uint32]*openconfig.Interface_Subinterface{
				0: {
					Index:   ygot.Uint32(0),
					Enabled: ygot.Bool(true),
					Ipv4: &openconfig.Interface_Subinterface_Ipv4{
						Address: map[string]*openconfig.Interface_Subinterface_Ipv4_Address{
							"10.1.4.54": {Ip: ygot.String("10.1.4.54"), PrefixLength: ygot.Uint8(21)},
						},
					},
				},
			},
		},
		"Loopback0": {
			Name:    ygot.String("Loopback0"),
			Enabled: ygot.Bool(true),
			Type:    openconfig.IETFInterfaces_InterfaceType_softwareLoopback,
			Subinterface: map[uint32]*openconfig.Interface_Subinterface{
				0: {
					Index:   ygot.Uint32(0),
					Enabled: ygot.Bool(true),
					Ipv4: &openconfig.Interface_Subinterface_Ipv4{
						Address: map[string]*openconfig.Interface_Subinterface_Ipv4_Address{
							"10.2.180.112": {Ip: ygot.String("10.2.180.112"), PrefixLength: ygot.Uint8(32)},
						},
					},
					Ipv6: &openconfig.Interface_Subinterface_Ipv6{
						Address: map[string]*openconfig.Interface_Subinterface_Ipv6_Address{
							"2001:db8:1::112": {Ip: ygot.String("2001:db8:1::112"), PrefixLength: ygot.Uint8(128)},
						},
					},
				},
			},
		},
		"console": {
			Name:    ygot.String("console"),
			Enabled: ygot.Bool(true),
			Subinterface: map[uint32]*openconfig.Interface_Subinterface{
				0: {
					Index:   ygot.Uint32(0),
					Enabled: ygot.Bool(true),
				},
			},
		},
	}

	wantBindings := ifconvertors.NetworkInstanceInterfaces{
		"prod": {
			"Ethernet0.0": {
				Id:           ygot.String("Ethernet0.0"),
				Interface:    ygot.String("Ethernet0"),
				Subinterface: ygot.Uint32(0),
			},
		},
		"default": {
			"vlan800.0": {
				Id:           ygot.String("vlan800.0"),
				Interface:    ygot.String("vlan800"),
				Subinterface: ygot.Uint32(0),
			},
			"Loopback0.0": {
				Id:           ygot.String("Loopback0.0"),
				Interface:    ygot.String("Loopback0"),
				Subinterface: ygot.Uint32(0),
			},
		},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unexpected interfaces diff: %s\n", diff)
	}
	if diff := cmp.Diff(gotBindings, wantBindings); diff != "" {
		t.Errorf("unexpected bindings diff: %s\n", diff)
	}
}

func TestInterfacesToOpenconfigEmpty(t *testing.T) {
	got, gotBindings, err := toOpenconfig(t, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no interface, got %d", len(got))
	}
	if len(gotBindings) != 0 {
		t.Errorf("expected no binding, got %d", len(gotBindings))
	}
}

func TestInterfacesToOpenconfigSwitchedVlanAccess(t *testing.T) {
	ethernet0 := newDeviceInterface("Ethernet0", true)

	unit0 := newLogicalInterface("Ethernet0", 0, cmdbif.TypeL2)
	unit0.Mode = cmdbif.ModeAccess
	unit0.UntaggedVlan = &cmdbif.VLANLite{Vid: 300, Name: "servers"}

	got, gotBindings, err := toOpenconfig(t, []*cmdbif.DeviceInterface{ethernet0}, []*cmdbif.LogicalInterface{unit0}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	want := &openconfig.Interface_Ethernet_SwitchedVlan{
		InterfaceMode: openconfig.VlanTypes_VlanModeType_ACCESS,
		AccessVlan:    ygot.Uint16(300),
	}
	if diff := cmp.Diff(got["Ethernet0"].Ethernet.SwitchedVlan, want); diff != "" {
		t.Errorf("unexpected switched-vlan diff: %s\n", diff)
	}
	if len(gotBindings) != 0 {
		t.Errorf("expected no network instance binding for a L2 interface, got %d", len(gotBindings))
	}
}

func TestInterfacesToOpenconfigSwitchedVlanTagged(t *testing.T) {
	lag := newDeviceInterface("PortChannel10", true)

	unit0 := newLogicalInterface("PortChannel10", 0, cmdbif.TypeL2)
	unit0.Mode = cmdbif.ModeTagged
	unit0.NativeVlan = &cmdbif.VLANLite{Vid: 100, Name: "native"}
	unit0.TaggedVlans = []*cmdbif.VLANLite{
		{Vid: 300, Name: "servers"},
		{Vid: 200, Name: "storage"},
		{Vid: 300, Name: "servers"}, // duplicate must be removed
	}

	got, _, err := toOpenconfig(t, []*cmdbif.DeviceInterface{lag}, []*cmdbif.LogicalInterface{unit0}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// the 802.1Q configuration of a LAG lands in the aggregation container
	want := &openconfig.Interface_Aggregation_SwitchedVlan{
		InterfaceMode: openconfig.VlanTypes_VlanModeType_TRUNK,
		NativeVlan:    ygot.Uint16(100),
		TrunkVlans: []openconfig.Interface_Aggregation_SwitchedVlan_TrunkVlans_Union{
			openconfig.UnionUint16(200),
			openconfig.UnionUint16(300),
		},
	}
	if diff := cmp.Diff(got["PortChannel10"].Aggregation.SwitchedVlan, want); diff != "" {
		t.Errorf("unexpected switched-vlan diff: %s\n", diff)
	}
	if got["PortChannel10"].Type != openconfig.IETFInterfaces_InterfaceType_ieee8023adLag {
		t.Errorf("unexpected interface type: %s", got["PortChannel10"].Type)
	}
}

func TestInterfacesToOpenconfigSubinterfaceMtu(t *testing.T) {
	ethernet0 := newDeviceInterface("Ethernet0", true)

	unit100 := newLogicalInterface("Ethernet0", 100, cmdbif.TypeL3)
	unit100.Mtu = 9000
	unit100.IPv4Address = newIPAddress("192.0.2.1/24", 4)

	got, _, err := toOpenconfig(t, []*cmdbif.DeviceInterface{ethernet0}, []*cmdbif.LogicalInterface{unit100}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	iface := got["Ethernet0"]
	if iface.Mtu != nil {
		t.Errorf("interface MTU must not be set from a non-zero unit, got %d", *iface.Mtu)
	}
	if mtu := iface.Subinterface[100].Ipv4.Mtu; mtu == nil || *mtu != 9000 {
		t.Errorf("unexpected IPv4 MTU: %v", mtu)
	}
}

func TestInterfacesToOpenconfigErrors(t *testing.T) {
	tests := []struct {
		name             string
		deviceInterfaces func() []*cmdbif.DeviceInterface
		logicalInterface func() *cmdbif.LogicalInterface
		wantError        string
	}{
		{
			name: "unknown parent interface",
			logicalInterface: func() *cmdbif.LogicalInterface {
				return newLogicalInterface("Ethernet4", 0, cmdbif.TypeL3)
			},
			wantError: "references unknown device interface",
		},
		{
			name: "missing index",
			logicalInterface: func() *cmdbif.LogicalInterface {
				logicalInterface := newLogicalInterface("Ethernet0", 0, cmdbif.TypeL3)
				logicalInterface.Index = nil
				return logicalInterface
			},
			wantError: "missing index",
		},
		{
			name: "inconsistent IPv4 family",
			logicalInterface: func() *cmdbif.LogicalInterface {
				logicalInterface := newLogicalInterface("Ethernet0", 0, cmdbif.TypeL3)
				logicalInterface.IPv4Address = newIPAddress("2001:db8::11/127", 4)
				return logicalInterface
			},
			wantError: "inconsistent IPv4 address",
		},
		{
			name: "inconsistent IPv6 family",
			logicalInterface: func() *cmdbif.LogicalInterface {
				logicalInterface := newLogicalInterface("Ethernet0", 0, cmdbif.TypeL3)
				logicalInterface.IPv6Address = newIPAddress("192.0.2.1/24", 6)
				return logicalInterface
			},
			wantError: "inconsistent IPv6 address",
		},
		{
			name: "access mode without untagged VLAN",
			logicalInterface: func() *cmdbif.LogicalInterface {
				logicalInterface := newLogicalInterface("Ethernet0", 0, cmdbif.TypeL2)
				logicalInterface.Mode = cmdbif.ModeAccess
				return logicalInterface
			},
			wantError: "no untagged VLAN",
		},
		{
			name: "802.1Q on a non Ethernet non LAG interface",
			deviceInterfaces: func() []*cmdbif.DeviceInterface {
				return []*cmdbif.DeviceInterface{newDeviceInterface("vlan800", true)}
			},
			logicalInterface: func() *cmdbif.LogicalInterface {
				logicalInterface := newLogicalInterface("vlan800", 0, cmdbif.TypeL2)
				logicalInterface.Mode = cmdbif.ModeAccess
				logicalInterface.UntaggedVlan = &cmdbif.VLANLite{Vid: 300, Name: "servers"}
				return logicalInterface
			},
			wantError: "neither an Ethernet nor a LAG",
		},
		{
			name: "invalid MTU",
			logicalInterface: func() *cmdbif.LogicalInterface {
				logicalInterface := newLogicalInterface("Ethernet0", 0, cmdbif.TypeL3)
				logicalInterface.Mtu = 100000
				return logicalInterface
			},
			wantError: "invalid MTU",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deviceInterfaces := []*cmdbif.DeviceInterface{newDeviceInterface("Ethernet0", true)}
			if test.deviceInterfaces != nil {
				deviceInterfaces = test.deviceInterfaces()
			}

			_, _, err := toOpenconfig(t, deviceInterfaces, []*cmdbif.LogicalInterface{test.logicalInterface()}, nil, nil)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}

func TestInterfacesToOpenconfigDuplicates(t *testing.T) {
	t.Run("duplicate device interface", func(t *testing.T) {
		_, _, err := toOpenconfig(t,
			[]*cmdbif.DeviceInterface{newDeviceInterface("Ethernet0", true), newDeviceInterface("Ethernet0", true)},
			nil,
			nil,
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "duplicate device interface") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("duplicate logical interface", func(t *testing.T) {
		_, _, err := toOpenconfig(t,
			[]*cmdbif.DeviceInterface{newDeviceInterface("Ethernet0", true)},
			[]*cmdbif.LogicalInterface{
				newLogicalInterface("Ethernet0", 0, cmdbif.TypeL3),
				newLogicalInterface("Ethernet0", 0, cmdbif.TypeL3),
			},
			nil,
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "duplicate logical interface") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func newPortLayout(logicalName, vendorName, vendorShortName string, lanes ...uint16) *cmdbif.PortLayout {
	port := cmdbif.PortLayout{
		Name:            vendorShortName,
		LogicalName:     logicalName,
		VendorName:      vendorName,
		VendorShortName: vendorShortName,
		Lanes:           lanes,
	}
	port.DeviceType.ID = 7
	port.NetworkRole.ID = 3
	return &port
}

func withIndex(port *cmdbif.PortLayout, index uint16) *cmdbif.PortLayout {
	port.Index = &index
	return port
}

// TestInterfacesToOpenconfigPortLayout ensures front-panel ports are renamed
// to their vendor name and carry the alias and lanes of the port layout, and
// that logical interfaces and network instance bindings follow the new name.
func TestInterfacesToOpenconfigPortLayout(t *testing.T) {
	uplink := newDeviceInterface("to_spine_01", true)
	uplink.Fec = "auto"

	noLanes := newDeviceInterface("to_spine_02", true) // lanes not documented in the layout

	loopback := newDeviceInterface("Loopback0", true) // not a front-panel port, not in the layout

	unit0 := newLogicalInterface("to_spine_01", 0, cmdbif.TypeL3)
	unit0.IPv4Address = newIPAddress("100.64.0.11/31", 4)
	unit0.UseIPv6LinkLocalOnly = ygot.Bool(true)

	noLanesUnit0 := newLogicalInterface("to_spine_02", 0, cmdbif.TypeL3)
	noLanesUnit0.UseIPv6LinkLocalOnly = ygot.Bool(false) // emitted as false

	breakout := newDeviceInterface("to_leaf_02", true)

	mgmt := newDeviceInterface("eth0", true) // management port, not a front-panel port, not in the layout

	portLayout := cmdbif.PortLayoutTable{
		"to_spine_01": withIndex(newPortLayout("to_spine_01", "Ethernet0", "etp0", 0, 1, 2, 3), 0),
		"to_spine_02": newPortLayout("to_spine_02", "Ethernet4", "etp1"),                     // no index
		"to_leaf_01":  withIndex(newPortLayout("to_leaf_01", "Ethernet8", "etp2a", 8, 9), 2), // no device interface
		"to_leaf_02":  withIndex(newPortLayout("to_leaf_02", "Ethernet10", "etp2b", 10, 11), 2),
	}

	got, gotBindings, err := toOpenconfig(t,
		[]*cmdbif.DeviceInterface{uplink, noLanes, loopback, breakout, mgmt},
		[]*cmdbif.LogicalInterface{unit0, noLanesUnit0},
		portLayout,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	want := map[string]*openconfig.Interface{
		"Ethernet0": {
			Name:                 ygot.String("Ethernet0"),
			Enabled:              ygot.Bool(true),
			Type:                 openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd,
			UseIpv6LinkLocalOnly: ygot.Bool(true),
			Ethernet: &openconfig.Interface_Ethernet{
				Alias:   ygot.String("etp0"),
				Lanes:   ygot.String("0,1,2,3"),
				Index:   ygot.Uint16(0),
				FecMode: openconfig.IfEthernet_INTERFACE_FEC_FEC_AUTO,
			},
			Subinterface: map[uint32]*openconfig.Interface_Subinterface{
				0: {
					Index:   ygot.Uint32(0),
					Enabled: ygot.Bool(true),
					Ipv4: &openconfig.Interface_Subinterface_Ipv4{
						Address: map[string]*openconfig.Interface_Subinterface_Ipv4_Address{
							"100.64.0.11": {Ip: ygot.String("100.64.0.11"), PrefixLength: ygot.Uint8(31)},
						},
					},
				},
			},
		},
		"Ethernet4": {
			Name:                 ygot.String("Ethernet4"),
			Enabled:              ygot.Bool(true),
			Type:                 openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd,
			UseIpv6LinkLocalOnly: ygot.Bool(false),
			Ethernet: &openconfig.Interface_Ethernet{
				Alias: ygot.String("etp1"), // no index in the port layout, none emitted
			},
			Subinterface: map[uint32]*openconfig.Interface_Subinterface{
				0: {
					Index:   ygot.Uint32(0),
					Enabled: ygot.Bool(true),
				},
			},
		},
		"Ethernet10": {
			Name:    ygot.String("Ethernet10"),
			Enabled: ygot.Bool(true),
			Type:    openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Ethernet: &openconfig.Interface_Ethernet{
				Alias: ygot.String("etp2b"),
				Lanes: ygot.String("10,11"),
				Index: ygot.Uint16(2), // breakout ports share the index of their parent port
			},
		},
		"eth0": {
			Name:       ygot.String("eth0"),
			Enabled:    ygot.Bool(true),
			Type:       openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Management: ygot.Bool(true),
		},
		"Loopback0": {
			Name:    ygot.String("Loopback0"),
			Enabled: ygot.Bool(true),
			Type:    openconfig.IETFInterfaces_InterfaceType_softwareLoopback,
		},
	}

	wantBindings := ifconvertors.NetworkInstanceInterfaces{
		"default": {
			"Ethernet0.0": {
				Id:           ygot.String("Ethernet0.0"),
				Interface:    ygot.String("Ethernet0"),
				Subinterface: ygot.Uint32(0),
			},
		},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unexpected interfaces diff: %s\n", diff)
	}
	if diff := cmp.Diff(gotBindings, wantBindings); diff != "" {
		t.Errorf("unexpected bindings diff: %s\n", diff)
	}
}

// TestInterfacesToOpenconfigNeighbors ensures the remote end of each link is
// emitted on the local port. The remote port name is resolved by the
// precompute: see TestPrecomputeNeighbors.
func TestInterfacesToOpenconfigNeighbors(t *testing.T) {
	uplink := newDeviceInterface("to_spine_01", true)
	mgmt := newDeviceInterface("eth0", true)
	unlinked := newDeviceInterface("to_spine_02", true)

	portLayout := cmdbif.PortLayoutTable{
		"to_spine_01": newPortLayout("to_spine_01", "Ethernet0", "etp1"),
		"to_spine_02": newPortLayout("to_spine_02", "Ethernet4", "etp2"),
	}
	neighbors := cmdbif.NeighborTable{
		"to_spine_01": {Device: "spine01-01", Port: "Ethernet8"},
		"eth0":        {Device: "oob01-01", Port: "to_tor01-01"}, // remote device without port layout
		"to_spine_02": nil,                                       // in several links
	}

	got, _, err := toOpenconfig(t, []*cmdbif.DeviceInterface{uplink, mgmt, unlinked}, nil, portLayout, neighbors)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	for name, want := range map[string][2]string{
		"Ethernet0": {"spine01-01", "Ethernet8"},
		"eth0":      {"oob01-01", "to_tor01-01"},
	} {
		iface := got[name]
		if iface == nil || iface.NeighborName == nil || iface.NeighborPort == nil {
			t.Fatalf("missing neighbor on %s: %+v", name, iface)
		}
		if *iface.NeighborName != want[0] || *iface.NeighborPort != want[1] {
			t.Errorf("unexpected neighbor on %s: %s:%s", name, *iface.NeighborName, *iface.NeighborPort)
		}
	}
	if iface := got["Ethernet4"]; iface.NeighborName != nil || iface.NeighborPort != nil {
		t.Errorf("unexpected neighbor on Ethernet4: %+v", iface)
	}

	json, err := ygot.EmitJSON(&openconfig.Device{Interface: map[string]*openconfig.Interface{"Ethernet0": got["Ethernet0"]}}, &ygot.EmitJSONConfig{Format: ygot.RFC7951})
	if err != nil {
		t.Fatalf("unable to emit JSON: %s", err)
	}
	json = strings.Join(strings.Fields(json), "")
	for _, leaf := range []string{`"neighbor-name":"spine01-01"`, `"neighbor-port":"Ethernet8"`} {
		if !strings.Contains(json, leaf) {
			t.Errorf("missing %s in %s", leaf, json)
		}
	}
}

// TestInterfacesToOpenconfigNeighborDescription ensures the neighbor port
// ending a description is named the way the remote device names it, and that
// other descriptions are kept as is.
func TestInterfacesToOpenconfigNeighborDescription(t *testing.T) {
	uplink := newDeviceInterface("Ethernet0", true)
	uplink.Description = "spine01-01:uplink_12"
	uplinkUnit0 := newLogicalInterface("Ethernet0", 0, "")
	uplinkUnit0.Description = "spine01-01:uplink_12"
	mgmt := newDeviceInterface("eth0", true)
	mgmt.Description = "oob01-01:to_tor01-01"
	custom := newDeviceInterface("Ethernet4", true)
	custom.Description = "to spine01-01"
	shortened := newDeviceInterface("Ethernet12", true)
	shortened.Description = "sp01 uplink_14"
	unlinked := newDeviceInterface("Ethernet8", true)
	unlinked.Description = "spine01-01:uplink_13"

	neighbors := cmdbif.NeighborTable{
		"Ethernet0": {Device: "spine01-01", Port: "Ethernet184", CmdbPort: "uplink_12"},
		// remote device without port layout
		"eth0":       {Device: "oob01-01", Port: "to_tor01-01", CmdbPort: "to_tor01-01"},
		"Ethernet4":  {Device: "spine01-01", Port: "Ethernet192", CmdbPort: "uplink_13"},
		"Ethernet12": {Device: "spine01-01", Port: "Ethernet200", CmdbPort: "uplink_14"},
	}

	got, _, err := toOpenconfig(t, []*cmdbif.DeviceInterface{uplink, mgmt, custom, unlinked, shortened}, []*cmdbif.LogicalInterface{uplinkUnit0}, nil, neighbors)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	for name, want := range map[string]string{
		"Ethernet0":  "spine01-01:Ethernet184",
		"eth0":       "oob01-01:to_tor01-01",
		"Ethernet4":  "to spine01-01",
		"Ethernet8":  "spine01-01:uplink_13",
		"Ethernet12": "sp01 Ethernet200",
	} {
		if iface := got[name]; iface == nil || iface.Description == nil || *iface.Description != want {
			t.Errorf("unexpected description on %s: want %q, got %+v", name, want, iface)
		}
	}
	if unit0 := got["Ethernet0"].Subinterface[0]; unit0 == nil || unit0.Description == nil || *unit0.Description != "spine01-01:Ethernet184" {
		t.Errorf("unexpected description on Ethernet0.0: %+v", unit0)
	}
}

func TestInterfacesToOpenconfigPortLayoutConflict(t *testing.T) {
	portLayout := cmdbif.PortLayoutTable{
		"to_spine_01": newPortLayout("to_spine_01", "Ethernet0", "etp1", 0, 1, 2, 3),
	}

	// a device interface already named like the vendor port of another one
	_, _, err := toOpenconfig(t,
		[]*cmdbif.DeviceInterface{newDeviceInterface("Ethernet0", true), newDeviceInterface("to_spine_01", true)},
		nil,
		portLayout,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "both map to port Ethernet0") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestInterfacesToOpenconfigPhysicalWithoutLayout ensures an interface named
// after its remote end on a device without port layout is still typed as
// Ethernet from its speed, so that its 802.1Q configuration can be emitted.
func TestInterfacesToOpenconfigPhysicalWithoutLayout(t *testing.T) {
	uplink := newDeviceInterface("to_I-RA", true)
	uplink.Speed = 40000

	unit0 := newLogicalInterface("to_I-RA", 0, cmdbif.TypeL2)
	unit0.Mode = cmdbif.ModeTagged
	unit0.TaggedVlans = []*cmdbif.VLANLite{{Vid: 810, Name: "servers"}}

	got, _, err := toOpenconfig(t, []*cmdbif.DeviceInterface{uplink}, []*cmdbif.LogicalInterface{unit0}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	iface := got["to_I-RA"]
	if iface.Type != openconfig.IETFInterfaces_InterfaceType_ethernetCsmacd {
		t.Errorf("unexpected type: %s", iface.Type)
	}
	if iface.Ethernet == nil || iface.Ethernet.PortSpeed != openconfig.IfEthernet_ETHERNET_SPEED_SPEED_40GB {
		t.Errorf("unexpected ethernet container: %+v", iface.Ethernet)
	}
	if iface.Ethernet.SwitchedVlan == nil || len(iface.Ethernet.SwitchedVlan.TrunkVlans) != 1 {
		t.Errorf("unexpected switched-vlan: %+v", iface.Ethernet.SwitchedVlan)
	}
}

func TestInterfacesToOpenconfigIPv6LinkLocalOnlyNonZeroUnit(t *testing.T) {
	ethernet0 := newDeviceInterface("Ethernet0", true)

	unit100 := newLogicalInterface("Ethernet0", 100, cmdbif.TypeL3)
	unit100.UseIPv6LinkLocalOnly = ygot.Bool(true)

	_, _, err := toOpenconfig(t, []*cmdbif.DeviceInterface{ethernet0}, []*cmdbif.LogicalInterface{unit100}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "only the logical unit 0 can carry it") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestInterfacesToOpenconfigIPv6LinkLocalOnlyFalseNonZeroUnit: false is the
// default behavior, accepted on any unit but only emitted from unit 0.
func TestInterfacesToOpenconfigIPv6LinkLocalOnlyFalseNonZeroUnit(t *testing.T) {
	ethernet0 := newDeviceInterface("Ethernet0", true)

	unit100 := newLogicalInterface("Ethernet0", 100, cmdbif.TypeL3)
	unit100.UseIPv6LinkLocalOnly = ygot.Bool(false)

	got, _, err := toOpenconfig(t, []*cmdbif.DeviceInterface{ethernet0}, []*cmdbif.LogicalInterface{unit100}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got["Ethernet0"].UseIpv6LinkLocalOnly != nil {
		t.Error("use-ipv6-link-local-only must not be set from a non-zero unit")
	}
}

func TestInterfacesToOpenconfigUnsupportedSpeed(t *testing.T) {
	ethernet0 := newDeviceInterface("Ethernet0", true)
	ethernet0.Speed = 123456

	_, _, err := toOpenconfig(t, []*cmdbif.DeviceInterface{ethernet0}, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported speed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestInterfacesToOpenconfigDeterministicJSON ensures the emitted JSON is
// schema-valid and deterministic: ygot/encoding-json sorts map keys, so two
// conversions of the same CMDB data must produce the same document.
func TestInterfacesToOpenconfigDeterministicJSON(t *testing.T) {
	build := func() string {
		ethernet2 := newDeviceInterface("Ethernet2", true)
		ethernet10 := newDeviceInterface("Ethernet10", true)
		ethernet0 := newDeviceInterface("Ethernet0", true)

		unit0 := newLogicalInterface("Ethernet0", 0, cmdbif.TypeL3)
		unit0.Vrf = &cmdbif.VRFLite{Name: "prod"}
		unit0.IPv4Address = newIPAddress("100.64.0.11/31", 4)

		interfaces, bindings, err := toOpenconfig(t,
			[]*cmdbif.DeviceInterface{ethernet2, ethernet10, ethernet0},
			[]*cmdbif.LogicalInterface{unit0},
			nil,
			nil,
		)
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
			}
		}

		out, err := ygot.EmitJSON(&device, &ygot.EmitJSONConfig{Format: ygot.RFC7951, SkipValidation: false})
		if err != nil {
			t.Fatalf("failed to emit JSON: %s", err)
		}
		return out
	}

	first := build()
	second := build()
	if first != second {
		t.Error("the emitted JSON is not deterministic")
	}

	// interface lists are emitted sorted by key
	posEthernet0 := strings.Index(first, `"name": "Ethernet0"`)
	posEthernet10 := strings.Index(first, `"name": "Ethernet10"`)
	posEthernet2 := strings.Index(first, `"name": "Ethernet2"`)
	if posEthernet0 == -1 || posEthernet10 == -1 || posEthernet2 == -1 {
		t.Fatal("missing interfaces in the emitted JSON")
	}
	if posEthernet0 >= posEthernet10 || posEthernet10 >= posEthernet2 {
		t.Error("interfaces are not emitted in a sorted order")
	}
}
