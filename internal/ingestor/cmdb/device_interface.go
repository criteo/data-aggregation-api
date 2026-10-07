package cmdb

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/ingestor/netbox"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
)

// GetDeviceInterfaces returns all device (physical) interfaces from the Network CMDB.
func GetDeviceInterfaces() ([]*interfaces.DeviceInterface, error) {
	response := netbox.NetboxResponse[interfaces.DeviceInterface]{}
	params := deviceDatacenterFilter()

	err := netbox.Get("/api/plugins/cmdb/device-interfaces/", &response, params)
	if err != nil {
		return nil, fmt.Errorf("device interfaces fetching failure: %w", err)
	}

	if response.Count != len(response.Results) {
		log.Warn().Msg("some device interfaces have not been fetched")
	}

	return response.Results, nil
}

// PrecomputeDeviceInterfaces associates each found device interface to the matching device.
func PrecomputeDeviceInterfaces(deviceInterfaces []*interfaces.DeviceInterface) map[string][]*interfaces.DeviceInterface {
	var interfacesPerDevice = make(map[string][]*interfaces.DeviceInterface)
	for _, deviceInterface := range deviceInterfaces {
		hostname := deviceInterface.Device.Name
		interfacesPerDevice[hostname] = append(interfacesPerDevice[hostname], deviceInterface)
	}

	return interfacesPerDevice
}
