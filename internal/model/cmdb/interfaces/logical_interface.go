package interfaces

import (
	"strconv"

	"github.com/criteo/data-aggregation-api/internal/types"
)

// Logical interface types as defined in the CMDB.
const (
	TypeL1 = "l1"
	TypeL2 = "l2"
	TypeL3 = "l3"
)

// Logical interface 802.1Q modes as defined in the CMDB.
const (
	ModeAccess = "access"
	ModeTagged = "tagged"
)

// IPAddress is an IP address assigned to a logical interface.
type IPAddress struct {
	Address types.CIDR `json:"address" validate:"required"`
	Family  int        `json:"family"  validate:"required"`
}

// VRFLite is a VRF as nested in logical interface responses.
type VRFLite struct {
	Name string `json:"name" validate:"required"`
}

// VLANLite is a VLAN as nested in logical interface responses.
type VLANLite struct {
	Vid  uint16 `json:"vid"  validate:"required"`
	Name string `json:"name" validate:"required"`
}

// LogicalInterface is a logical unit of a device interface in the Network CMDB.
//
// CMDB endpoint: /api/plugins/cmdb/logical-interfaces/.
type LogicalInterface struct {
	ParentInterface struct {
		Name   string `json:"name" validate:"required"`
		Device struct {
			Name string `json:"name" validate:"required"`
		} `json:"device" validate:"required"`
	} `json:"parent_interface" validate:"required"`

	Index       *uint32    `json:"index"          validate:"required"`
	Enabled     *bool      `json:"enabled"        validate:"required"`
	Type        string     `json:"type"           validate:"required,oneof=l1 l2 l3"`
	Mtu         uint32     `json:"mtu"            validate:"omitempty"`
	Vrf         *VRFLite   `json:"vrf"            validate:"omitempty"`
	IPv4Address *IPAddress `json:"ipv4_address"   validate:"omitempty"`
	IPv6Address *IPAddress `json:"ipv6_address"   validate:"omitempty"`
	// UseIPv6LinkLocalOnly addresses the interface with its IPv6 link-local
	// address only, as BGP unnumbered peering does.
	UseIPv6LinkLocalOnly *bool       `json:"use_ipv6_link_local_only" validate:"omitempty"`
	Mode                 string      `json:"mode"           validate:"omitempty,oneof=access tagged"`
	UntaggedVlan         *VLANLite   `json:"untagged_vlan"  validate:"omitempty"`
	TaggedVlans          []*VLANLite `json:"tagged_vlans"   validate:"omitempty,dive"`
	NativeVlan           *VLANLite   `json:"native_vlan"    validate:"omitempty"`
	Description          string      `json:"description"    validate:"omitempty"`
}

// Name returns the conventional display name of the logical interface,
// e.g. "Ethernet0.0", following the CMDB convention.
func (l *LogicalInterface) Name() string {
	if l.Index == nil {
		return l.ParentInterface.Name
	}
	return l.ParentInterface.Name + "." + strconv.FormatUint(uint64(*l.Index), 10)
}
