package interfaces

import (
	"strconv"
	"strings"
)

// PortLayoutKey identifies a port layout: the front-panel ports of one
// hardware model used in one network role.
type PortLayoutKey struct {
	DeviceTypeID int
	RoleID       int
}

// PortLayout is a front-panel port of a hardware model in a network role,
// with the names the port bears in the NOS and the ASIC lanes backing it.
//
// CMDB endpoint: /api/plugins/cmdb/port-layouts/.
type PortLayout struct {
	DeviceType struct {
		ID    int    `json:"id"    validate:"required"`
		Model string `json:"model" validate:"omitempty"`
	} `json:"device_type" validate:"required"`
	NetworkRole struct {
		ID   int    `json:"id"   validate:"required"`
		Name string `json:"name" validate:"omitempty"`
	} `json:"network_role" validate:"required"`

	// Name is the generic port name (e.g. etp1a), LogicalName the
	// vendor-agnostic name the CMDB device interfaces use (e.g. to_L3-SP01).
	Name        string `json:"name"         validate:"required"`
	LabelName   string `json:"label_name"   validate:"omitempty"`
	LogicalName string `json:"logical_name" validate:"omitempty"`
	// VendorName is the port name in the NOS (e.g. Ethernet0),
	// VendorShortName its alias (e.g. etp1).
	VendorName      string `json:"vendor_name"       validate:"omitempty"`
	VendorShortName string `json:"vendor_short_name" validate:"omitempty"`
	VendorLongName  string `json:"vendor_long_name"  validate:"omitempty"`
	// Lanes are the ASIC lanes of the port in hardware order, empty when not documented.
	Lanes []uint16 `json:"lanes" validate:"omitempty"`
}

// Key returns the layout the port belongs to.
func (p *PortLayout) Key() PortLayoutKey {
	return PortLayoutKey{DeviceTypeID: p.DeviceType.ID, RoleID: p.NetworkRole.ID}
}

// LanesString returns the lanes as the comma-separated list SONiC expects
// (e.g. "0,1,2,3"), empty when the lanes are not documented.
func (p *PortLayout) LanesString() string {
	if len(p.Lanes) == 0 {
		return ""
	}
	lanes := make([]string, len(p.Lanes))
	for i, lane := range p.Lanes {
		lanes[i] = strconv.FormatUint(uint64(lane), 10)
	}
	return strings.Join(lanes, ",")
}

// PortLayoutTable is the port layout of one device: ports indexed by the
// names the CMDB device interfaces use.
type PortLayoutTable map[string]*PortLayout
