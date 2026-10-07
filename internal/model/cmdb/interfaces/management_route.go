package interfaces

import (
	"net"

	"github.com/criteo/data-aggregation-api/internal/types"
)

// Management route kinds as defined in the CMDB.
const (
	// ManagementRouteStatic is the default route of the management routing
	// table, with the management gateway as next-hop.
	ManagementRouteStatic = "static"
	// ManagementRouteForced is a destination steered through the management
	// routing table (SONiC forced_mgmt_routes), resolved by the static route.
	ManagementRouteForced = "forced"
)

// ManagementRoute is a route of the management routing table of a device,
// attached to its management logical interface.
//
// CMDB endpoint: /api/plugins/cmdb/management-routes/.
type ManagementRoute struct {
	LogicalInterface struct {
		Index           *uint32 `json:"index" validate:"required"`
		ParentInterface struct {
			Name   string `json:"name" validate:"required"`
			Device struct {
				Name string `json:"name" validate:"required"`
			} `json:"device" validate:"required"`
		} `json:"parent_interface" validate:"required"`
	} `json:"logical_interface" validate:"required"`

	Kind   string     `json:"kind"   validate:"required,oneof=static forced"`
	Prefix types.CIDR `json:"prefix" validate:"required"`
	// NextHop is the gateway of a static route, nil for a forced route.
	NextHop *net.IP `json:"next_hop" validate:"omitempty"`
}

// InterfaceName returns the name of the device interface the route is attached to.
func (m *ManagementRoute) InterfaceName() string {
	return m.LogicalInterface.ParentInterface.Name
}

// SubinterfaceIndex returns the index of the logical interface the route is attached to.
func (m *ManagementRoute) SubinterfaceIndex() uint32 {
	if m.LogicalInterface.Index == nil {
		return 0
	}
	return *m.LogicalInterface.Index
}
