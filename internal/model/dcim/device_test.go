package dcim_test

import (
	"encoding/json"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/criteo/data-aggregation-api/internal/ingestor/netbox"
	"github.com/criteo/data-aggregation-api/internal/model/dcim"
)

// validate checks a /api/dcim/devices/ response the way netbox.Get does.
func validate(t *testing.T, results string) error {
	t.Helper()
	var response netbox.NetboxResponse[dcim.NetworkDevice]
	if err := json.Unmarshal([]byte(`{"results": `+results+`}`), &response); err != nil {
		t.Fatal(err)
	}
	return validator.New().Struct(&response)
}

// TestNetworkDevice_TypeAndRoleAreOptional: a device without them stays in
// the inventory; it only lacks its DEVICE_METADATA.
func TestNetworkDevice_TypeAndRoleAreOptional(t *testing.T) {
	for name, results := range map[string]string{
		"complete": `[{"name": "tor01-01",
		  "device_type": {"id": 10, "model": "MSN2700"},
		  "device_role": {"id": 20, "name": "ToR"}}]`,
		"no device type": `[{"name": "tor01-01", "device_role": {"id": 20, "name": "ToR"}}]`,
		"no device role": `[{"name": "tor01-01", "device_type": {"id": 10, "model": "MSN2700"}}]`,
	} {
		if err := validate(t, results); err != nil {
			t.Errorf("%s: the device was rejected: %v", name, err)
		}
	}
}
