package cmdb_test

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/criteo/data-aggregation-api/internal/ingestor/cmdb"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/criteo/data-aggregation-api/internal/types"
	"github.com/google/go-cmp/cmp"
)

const logicalInterfacesJSON = `
[
    {
        "id": 1369,
        "parent_interface": {
            "id": 1245,
            "name": "Ethernet0",
            "device": {
                "id": 189760,
                "name": "tor01-01"
            }
        },
        "vrf": {
            "id": 1,
            "name": "prod"
        },
        "ipv4_address": {
            "id": 1,
            "url": "http://127.0.0.1:8001/api/ipam/ip-addresses/1/",
            "display": "100.64.0.11/31",
            "family": 4,
            "address": "100.64.0.11/31"
        },
        "ipv6_address": {
            "id": 2,
            "url": "http://127.0.0.1:8001/api/ipam/ip-addresses/2/",
            "display": "2001:db8::11/127",
            "family": 6,
            "address": "2001:db8::11/127"
        },
        "untagged_vlan": null,
        "tagged_vlans": [],
        "native_vlan": null,
        "display": "Ethernet0--0",
        "created": "2026-08-17T13:20:16.085380Z",
        "last_updated": "2026-08-17T13:20:16.085389Z",
        "index": 0,
        "enabled": true,
        "state": "staging",
        "monitoring_state": "disabled",
        "mtu": 9216,
        "type": "l3",
        "use_ipv6_link_local_only": true,
        "mode": null,
        "description": null
    },
    {
        "id": 1370,
        "parent_interface": {
            "id": 1246,
            "name": "Ethernet8",
            "device": {
                "id": 189761,
                "name": "spine01-01"
            }
        },
        "vrf": null,
        "ipv4_address": null,
        "ipv6_address": null,
        "untagged_vlan": null,
        "tagged_vlans": [
            {
                "id": 10,
                "vid": 300,
                "name": "servers",
                "description": "",
                "tenant": null
            },
            {
                "id": 11,
                "vid": 200,
                "name": "storage",
                "description": "",
                "tenant": null
            }
        ],
        "native_vlan": {
            "id": 12,
            "vid": 100,
            "name": "native",
            "description": "",
            "tenant": null
        },
        "display": "Ethernet8--100",
        "created": "2026-08-17T13:20:16.085380Z",
        "last_updated": "2026-08-17T13:20:16.085389Z",
        "index": 100,
        "enabled": false,
        "state": "staging",
        "monitoring_state": "disabled",
        "mtu": null,
        "type": "l2",
        "use_ipv6_link_local_only": false,
        "mode": "tagged",
        "description": "storage trunk"
    }
]
`

func TestPrecomputeLogicalInterfaces(t *testing.T) {
	var flagTrue = true
	var flagFalse = false
	var index0 uint32
	var index100 uint32 = 100

	want := map[string][]*interfaces.LogicalInterface{
		"tor01-01": {
			{
				ParentInterface: struct {
					Name   string `json:"name" validate:"required"`
					Device struct {
						Name string `json:"name" validate:"required"`
					} `json:"device" validate:"required"`
				}{
					Name: "Ethernet0",
					Device: struct {
						Name string `json:"name" validate:"required"`
					}{Name: "tor01-01"},
				},
				Index:   &index0,
				Enabled: &flagTrue,
				Type:    "l3",
				Mtu:     9216,
				Vrf:     &interfaces.VRFLite{Name: "prod"},
				IPv4Address: &interfaces.IPAddress{
					Address: types.CIDR{IP: net.ParseIP("100.64.0.11"), Netmask: 31},
					Family:  4,
				},
				IPv6Address: &interfaces.IPAddress{
					Address: types.CIDR{IP: net.ParseIP("2001:db8::11"), Netmask: 127},
					Family:  6,
				},
				UseIPv6LinkLocalOnly: &flagTrue,
				TaggedVlans:          []*interfaces.VLANLite{},
			},
		},
		"spine01-01": {
			{
				ParentInterface: struct {
					Name   string `json:"name" validate:"required"`
					Device struct {
						Name string `json:"name" validate:"required"`
					} `json:"device" validate:"required"`
				}{
					Name: "Ethernet8",
					Device: struct {
						Name string `json:"name" validate:"required"`
					}{Name: "spine01-01"},
				},
				Index:                &index100,
				Enabled:              &flagFalse,
				Type:                 "l2",
				UseIPv6LinkLocalOnly: &flagFalse,
				Mode:                 "tagged",
				TaggedVlans: []*interfaces.VLANLite{
					{Vid: 300, Name: "servers"},
					{Vid: 200, Name: "storage"},
				},
				NativeVlan:  &interfaces.VLANLite{Vid: 100, Name: "native"},
				Description: "storage trunk",
			},
		},
	}

	var cmdbOutput []*interfaces.LogicalInterface
	if err := json.Unmarshal([]byte(logicalInterfacesJSON), &cmdbOutput); err != nil {
		t.Fatalf("unable to load test data: %s", err)
	}

	out := cmdb.PrecomputeLogicalInterfaces(cmdbOutput)
	if diff := cmp.Diff(out, want); diff != "" {
		t.Errorf("unexpected diff: %s\n", diff)
	}
}

func TestLogicalInterfaceName(t *testing.T) {
	var index100 uint32 = 100

	logicalInterface := interfaces.LogicalInterface{Index: &index100}
	logicalInterface.ParentInterface.Name = "Ethernet8"

	if name := logicalInterface.Name(); name != "Ethernet8.100" {
		t.Errorf("unexpected name: %s", name)
	}
}
