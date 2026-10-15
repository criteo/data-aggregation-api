package interfaces

// LinkEndpoint is one end of a CMDB link: a device interface.
type LinkEndpoint struct {
	Name   string `json:"name" validate:"required"`
	Device struct {
		Name string `json:"name" validate:"required"`
	} `json:"device" validate:"required"`
}

// Link is a point-to-point link between two device interfaces in the Network CMDB.
//
// CMDB endpoint: /api/plugins/cmdb/links/.
type Link struct {
	InterfaceA LinkEndpoint `json:"interface_a" validate:"required"`
	InterfaceB LinkEndpoint `json:"interface_b" validate:"required"`
}

// Neighbor is the remote end of the link connected to a device interface.
type Neighbor struct {
	// Device is the hostname of the remote device.
	Device string
	// Interface is the CMDB name of the remote device interface (e.g. to_L3-SP01).
	Interface string
	// Port is the port layout entry of the remote interface, nil when the
	// remote device has no port layout or the interface is not in it.
	Port *PortLayout
}

// NeighborTable is the neighbors of one device, indexed by the CMDB name of
// the local device interface.
type NeighborTable map[string]*Neighbor
