package interfaces

// DeviceInterface is a physical interface of a device in the Network CMDB.
//
// CMDB endpoint: /api/plugins/cmdb/device-interfaces/.
type DeviceInterface struct {
	Device struct {
		Name string `json:"name" validate:"required"`
	} `json:"device" validate:"required"`

	Name            string `json:"name"            validate:"required"`
	Enabled         *bool  `json:"enabled"         validate:"required"`
	Autonegotiation *bool  `json:"autonegotiation" validate:"omitempty"`
	// Speed is expressed in Mb/s (e.g. 100000 for 100G), as stored in the CMDB.
	Speed       uint32 `json:"speed"       validate:"omitempty"`
	Fec         string `json:"fec"         validate:"omitempty,oneof=auto rs fc"`
	Description string `json:"description" validate:"omitempty"`
}
