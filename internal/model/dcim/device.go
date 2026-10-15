package dcim

type NetworkDevice struct {
	Hostname     string `json:"name" validate:"required"`
	SerialNumber string `json:"serial" validate:"omitempty"`
	Tags         []struct {
		Name string `json:"name" validate:"required"`
	} `json:"tags" validate:"omitempty"`
	// DeviceType and DeviceRole select the SONiC HwSKU and device type of
	// DEVICE_METADATA, through the CMDB mappings, and identify the port layout
	// of the device. Not validated: a device without them only lacks its
	// DEVICE_METADATA and interfaces, not its inventory entry.
	// NetBox exposes the role as "role" since 3.6 and as "device_role" before,
	// both are accepted: see RoleID().
	DeviceType struct {
		ID    int    `json:"id"`
		Model string `json:"model"`
	} `json:"device_type"`
	Role struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"role"`
	DeviceRole struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"device_role"`
}

// DeviceTypeID returns the NetBox identifier of the device type, 0 when unknown.
func (d *NetworkDevice) DeviceTypeID() int {
	return d.DeviceType.ID
}

// RoleID returns the NetBox identifier of the device role, 0 when unknown.
func (d *NetworkDevice) RoleID() int {
	if d.Role.ID != 0 {
		return d.Role.ID
	}
	return d.DeviceRole.ID
}
