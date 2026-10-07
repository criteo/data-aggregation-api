package cmdb

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/ingestor/netbox"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
)

// GetManagementRoutes returns all management routes from the Network CMDB.
func GetManagementRoutes() ([]*interfaces.ManagementRoute, error) {
	response := netbox.NetboxResponse[interfaces.ManagementRoute]{}

	// management routes are attached to a device via the parent interface of their logical interface
	params := prefixedDeviceDatacenterFilter("logical_interface__parent_interface__")

	err := netbox.Get("/api/plugins/cmdb/management-routes/", &response, params)
	if err != nil {
		return nil, fmt.Errorf("management routes fetching failure: %w", err)
	}

	if response.Count != len(response.Results) {
		log.Warn().Msg("some management routes have not been fetched")
	}

	return response.Results, nil
}

// PrecomputeManagementRoutes associates each found management route to the matching device.
func PrecomputeManagementRoutes(managementRoutes []*interfaces.ManagementRoute) map[string][]*interfaces.ManagementRoute {
	var routesPerDevice = make(map[string][]*interfaces.ManagementRoute)
	for _, route := range managementRoutes {
		hostname := route.LogicalInterface.ParentInterface.Device.Name
		routesPerDevice[hostname] = append(routesPerDevice[hostname], route)
	}

	return routesPerDevice
}
