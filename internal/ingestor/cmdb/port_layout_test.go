package cmdb_test

import (
	"encoding/json"
	"testing"

	"github.com/criteo/data-aggregation-api/internal/ingestor/cmdb"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
)

const portLayoutsJSON = `
[
    {
        "id": 1,
        "device_type": {
            "id": 7,
            "url": "https://netbox/api/dcim/device-types/7/",
            "display": "Vendor Model",
            "manufacturer": {"id": 2, "name": "Vendor", "slug": "vendor"},
            "model": "Model",
            "slug": "model"
        },
        "network_role": {
            "id": 3,
            "url": "https://netbox/api/dcim/device-roles/3/",
            "display": "ToR",
            "name": "ToR",
            "slug": "tor"
        },
        "display": "Vendor Model--ToR--etp1",
        "name": "etp1",
        "label_name": "1",
        "logical_name": "to_spine_01",
        "vendor_name": "Ethernet0",
        "vendor_short_name": "etp1",
        "vendor_long_name": "Ethernet0",
        "lanes": [0, 1, 2, 3]
    },
    {
        "id": 2,
        "device_type": {"id": 7, "model": "Model", "slug": "model"},
        "network_role": {"id": 3, "name": "ToR", "slug": "tor"},
        "name": "etp2",
        "label_name": "2",
        "logical_name": "to_spine_02",
        "vendor_name": "Ethernet4",
        "vendor_short_name": "etp2",
        "vendor_long_name": "Ethernet4",
        "lanes": []
    },
    {
        "id": 4,
        "device_type": {"id": 7, "model": "Model", "slug": "model"},
        "network_role": {"id": 3, "name": "ToR", "slug": "tor"},
        "name": "eth0",
        "label_name": "mgmt",
        "logical_name": "eth0",
        "vendor_name": "eth0",
        "vendor_short_name": "eth0",
        "vendor_long_name": "eth0",
        "lanes": []
    },
    {
        "id": 3,
        "device_type": {"id": 7, "model": "Model", "slug": "model"},
        "network_role": {"id": 4, "name": "Spine", "slug": "spine"},
        "name": "etp1",
        "label_name": "1",
        "logical_name": "to_leaf_01",
        "vendor_name": "Ethernet0",
        "vendor_short_name": "etp1",
        "vendor_long_name": "Ethernet0",
        "lanes": [4, 5, 6, 7]
    }
]
`

func TestPrecomputePortLayouts(t *testing.T) {
	var cmdbOutput []*interfaces.PortLayout
	if err := json.Unmarshal([]byte(portLayoutsJSON), &cmdbOutput); err != nil {
		t.Fatalf("unable to load test data: %s", err)
	}

	out := cmdb.PrecomputePortLayouts(cmdbOutput)

	if len(out) != 2 {
		t.Fatalf("expected 2 layouts, got %d", len(out))
	}

	tor := out[interfaces.PortLayoutKey{DeviceTypeID: 7, RoleID: 3}]
	if tor == nil {
		t.Fatal("missing ToR layout")
	}

	// ports are reachable by logical name and by generic name
	for _, name := range []string{"to_spine_01", "etp1"} {
		port, ok := tor[name]
		if !ok {
			t.Fatalf("missing port %s in the ToR layout", name)
		}
		if port.VendorName != "Ethernet0" || port.VendorShortName != "etp1" {
			t.Errorf("unexpected port for %s: %+v", name, port)
		}
		if lanes := port.LanesString(); lanes != "0,1,2,3" {
			t.Errorf("unexpected lanes for %s: %q", name, lanes)
		}
	}

	// a port named after itself is reachable once
	if port := tor["eth0"]; port == nil || port.VendorName != "eth0" {
		t.Errorf("unexpected port for eth0: %+v", port)
	}
	if len(tor) != 5 {
		t.Errorf("expected 5 entries in the ToR layout, got %d", len(tor))
	}

	if lanes := tor["to_spine_02"].LanesString(); lanes != "" {
		t.Errorf("expected no lanes for to_spine_02, got %q", lanes)
	}

	// the same hardware in another role is another layout
	spine := out[interfaces.PortLayoutKey{DeviceTypeID: 7, RoleID: 4}]
	if spine == nil || spine["to_leaf_01"] == nil {
		t.Fatal("missing Spine layout")
	}
	if lanes := spine["to_leaf_01"].LanesString(); lanes != "4,5,6,7" {
		t.Errorf("unexpected lanes for to_leaf_01: %q", lanes)
	}
}

func TestPrecomputePortLayoutsEmpty(t *testing.T) {
	out := cmdb.PrecomputePortLayouts(nil)
	if len(out) != 0 {
		t.Errorf("expected empty result, got %d entries", len(out))
	}
}
