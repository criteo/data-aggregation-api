package cmdb

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/ingestor/netbox"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
)

// GetLogicalInterfaces returns all logical interfaces from the Network CMDB.
func GetLogicalInterfaces() ([]*interfaces.LogicalInterface, error) {
	response := netbox.NetboxResponse[interfaces.LogicalInterface]{}

	// logical interfaces are attached to a device via their parent interface
	params := prefixedDeviceDatacenterFilter("parent_interface__")

	err := netbox.Get("/api/plugins/cmdb/logical-interfaces/", &response, params)
	if err != nil {
		return nil, fmt.Errorf("logical interfaces fetching failure: %w", err)
	}

	if response.Count != len(response.Results) {
		log.Warn().Msg("some logical interfaces have not been fetched")
	}

	return response.Results, nil
}

// PrecomputeLogicalInterfaces associates each found logical interface to the matching device.
func PrecomputeLogicalInterfaces(logicalInterfaces []*interfaces.LogicalInterface) map[string][]*interfaces.LogicalInterface {
	var interfacesPerDevice = make(map[string][]*interfaces.LogicalInterface)
	for _, logicalInterface := range logicalInterfaces {
		hostname := logicalInterface.ParentInterface.Device.Name
		interfacesPerDevice[hostname] = append(interfacesPerDevice[hostname], logicalInterface)
	}

	return interfacesPerDevice
}
