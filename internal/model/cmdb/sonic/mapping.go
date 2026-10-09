package sonic

// NotProvisioned is the SONiC device type of a role not mapped to a real one
// yet: it is no type at all for DEVICE_METADATA.
const NotProvisioned = "not-provisioned"

// HwskuMapping maps a DCIM device type (model) to a SONiC HwSKU.
type HwskuMapping struct {
	DeviceType struct {
		ID    int    `json:"id" validate:"required"`
		Model string `json:"model"`
	} `json:"device_type" validate:"required"`
	Hwsku string `json:"hwsku" validate:"required"`
}

// RoleMapping maps a DCIM device role to a SONiC device type.
type RoleMapping struct {
	DeviceRole struct {
		ID   int    `json:"id" validate:"required"`
		Name string `json:"name"`
	} `json:"device_role" validate:"required"`
	SONiCType string `json:"sonic_type" validate:"required"`
}
