package cmdb

import (
	"fmt"
	"net/url"

	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/ingestor/netbox"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/sonic"
)

// GetSONiCHwskuMappings returns the DCIM device type to SONiC HwSKU mappings.
// They are global, so not filtered on the datacenter.
func GetSONiCHwskuMappings() ([]*sonic.HwskuMapping, error) {
	response := netbox.NetboxResponse[sonic.HwskuMapping]{}

	err := netbox.Get("/api/plugins/cmdb/sonic-hwsku-mapping/", &response, url.Values{})
	if err != nil {
		return nil, fmt.Errorf("SONiC HwSKU mapping fetching failure: %w", err)
	}

	if len(response.Results) == 0 {
		log.Warn().Msg("no SONiC HwSKU mapping found")
	}

	return response.Results, nil
}

// GetSONiCRoleMappings returns the DCIM device role to SONiC device type
// mappings. They are global, so not filtered on the datacenter.
func GetSONiCRoleMappings() ([]*sonic.RoleMapping, error) {
	response := netbox.NetboxResponse[sonic.RoleMapping]{}

	err := netbox.Get("/api/plugins/cmdb/sonic-role-mapping/", &response, url.Values{})
	if err != nil {
		return nil, fmt.Errorf("SONiC role mapping fetching failure: %w", err)
	}

	if len(response.Results) == 0 {
		log.Warn().Msg("no SONiC role mapping found")
	}

	return response.Results, nil
}

// PrecomputeSONiCHwsku indexes the HwSKUs by DCIM device type ID.
func PrecomputeSONiCHwsku(mappings []*sonic.HwskuMapping) map[int]string {
	hwskuPerDeviceType := make(map[int]string, len(mappings))
	for _, mapping := range mappings {
		hwskuPerDeviceType[mapping.DeviceType.ID] = mapping.Hwsku
	}

	return hwskuPerDeviceType
}

// PrecomputeSONiCType indexes the SONiC device types by DCIM device role ID.
// A role still "not-provisioned" has no type, and is left out.
func PrecomputeSONiCType(mappings []*sonic.RoleMapping) map[int]string {
	typePerRole := make(map[int]string, len(mappings))
	for _, mapping := range mappings {
		if mapping.SONiCType == sonic.NotProvisioned {
			continue
		}
		typePerRole[mapping.DeviceRole.ID] = mapping.SONiCType
	}

	return typePerRole
}
