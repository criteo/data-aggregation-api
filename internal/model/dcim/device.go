package dcim

type NetworkDevice struct {
	Hostname     string `json:"name" validate:"required"`
	SerialNumber string `json:"serial" validate:"omitempty"`
	Tags         []struct {
		Name string `json:"name" validate:"required"`
	} `json:"tags" validate:"omitempty"`
	// DeviceType and DeviceRole select the SONiC HwSKU and device type of
	// DEVICE_METADATA, through the CMDB mappings. Not validated: a device
	// without them only lacks its DEVICE_METADATA, not its inventory entry.
	DeviceType struct {
		ID    int    `json:"id"`
		Model string `json:"model"`
	} `json:"device_type"`
	DeviceRole struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"device_role"`
}
