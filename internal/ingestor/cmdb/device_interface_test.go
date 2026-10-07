package cmdb_test

import (
	"encoding/json"
	"testing"

	"github.com/criteo/data-aggregation-api/internal/ingestor/cmdb"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/google/go-cmp/cmp"
)

const deviceInterfacesJSON = `
[
    {
        "id": 1245,
        "device": {
            "id": 189760,
            "name": "tor01-01"
        },
        "display": "tor01-01--Ethernet0",
        "created": "2026-08-17T13:20:15.959710Z",
        "last_updated": "2026-08-17T13:20:15.959718Z",
        "name": "Ethernet0",
        "enabled": true,
        "state": "staging",
        "monitoring_state": "disabled",
        "autonegotiation": true,
        "speed": 100000,
        "fec": "rs",
        "description": "LOCAL:spine01-01:Ethernet0"
    },
    {
        "id": 1246,
        "device": {
            "id": 189760,
            "name": "tor01-01"
        },
        "display": "tor01-01--Ethernet8",
        "created": "2026-08-17T13:20:15.959710Z",
        "last_updated": "2026-08-17T13:20:15.959718Z",
        "name": "Ethernet8",
        "enabled": false,
        "state": "staging",
        "monitoring_state": "disabled",
        "autonegotiation": true,
        "speed": null,
        "fec": null,
        "description": null
    },
    {
        "id": 1247,
        "device": {
            "id": 189761,
            "name": "spine01-01"
        },
        "display": "spine01-01--Ethernet0",
        "created": "2026-08-17T13:20:15.959710Z",
        "last_updated": "2026-08-17T13:20:15.959718Z",
        "name": "Ethernet0",
        "enabled": true,
        "state": "staging",
        "monitoring_state": "disabled",
        "autonegotiation": false,
        "speed": 400000,
        "fec": "fc",
        "description": null
    },
    {
        "id": 1248,
        "device": {
            "id": 189761,
            "name": "spine01-01"
        },
        "display": "spine01-01--Ethernet8",
        "created": "2026-08-17T13:20:15.959710Z",
        "last_updated": "2026-08-17T13:20:15.959718Z",
        "name": "Ethernet8",
        "enabled": true,
        "state": "staging",
        "monitoring_state": "disabled",
        "autonegotiation": false,
        "speed": 400000,
        "fec": "auto",
        "description": null
    }
]
`

func TestPrecomputeDeviceInterfaces(t *testing.T) {
	var flagTrue = true
	var flagFalse = false

	want := map[string][]*interfaces.DeviceInterface{
		"tor01-01": {
			{
				Device: struct {
					Name string `json:"name" validate:"required"`
				}{Name: "tor01-01"},
				Name:            "Ethernet0",
				Enabled:         &flagTrue,
				Autonegotiation: &flagTrue,
				Speed:           100000,
				Fec:             "rs",
				Description:     "LOCAL:spine01-01:Ethernet0",
			},
			{
				Device: struct {
					Name string `json:"name" validate:"required"`
				}{Name: "tor01-01"},
				Name:            "Ethernet8",
				Enabled:         &flagFalse,
				Autonegotiation: &flagTrue,
			},
		},
		"spine01-01": {
			{
				Device: struct {
					Name string `json:"name" validate:"required"`
				}{Name: "spine01-01"},
				Name:            "Ethernet0",
				Enabled:         &flagTrue,
				Autonegotiation: &flagFalse,
				Speed:           400000,
				Fec:             "fc",
			},
			{
				Device: struct {
					Name string `json:"name" validate:"required"`
				}{Name: "spine01-01"},
				Name:            "Ethernet8",
				Enabled:         &flagTrue,
				Autonegotiation: &flagFalse,
				Speed:           400000,
				Fec:             "auto",
			},
		},
	}

	var cmdbOutput []*interfaces.DeviceInterface
	if err := json.Unmarshal([]byte(deviceInterfacesJSON), &cmdbOutput); err != nil {
		t.Fatalf("unable to load test data: %s", err)
	}

	out := cmdb.PrecomputeDeviceInterfaces(cmdbOutput)
	if diff := cmp.Diff(out, want); diff != "" {
		t.Errorf("unexpected diff: %s\n", diff)
	}
}

func TestPrecomputeDeviceInterfacesEmpty(t *testing.T) {
	out := cmdb.PrecomputeDeviceInterfaces(nil)
	if len(out) != 0 {
		t.Errorf("expected empty result, got %d entries", len(out))
	}
}
