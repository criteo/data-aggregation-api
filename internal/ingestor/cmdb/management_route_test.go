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

const managementRoutesJSON = `
[
    {
        "id": 1,
        "logical_interface": {
            "id": 1369,
            "index": 0,
            "name": "eth0.0",
            "parent_interface": {
                "id": 1245,
                "name": "eth0",
                "device": {
                    "id": 189760,
                    "name": "tor01-01"
                }
            }
        },
        "display": "tor01-01--eth0.0--static--0.0.0.0/0",
        "created": "2026-10-07T13:20:16.085380Z",
        "last_updated": "2026-10-07T13:20:16.085389Z",
        "kind": "static",
        "prefix": "0.0.0.0/0",
        "next_hop": "10.0.0.1",
        "state": "staging",
        "monitoring_state": "disabled",
        "description": null
    },
    {
        "id": 2,
        "logical_interface": {
            "id": 1369,
            "index": 0,
            "name": "eth0.0",
            "parent_interface": {
                "id": 1245,
                "name": "eth0",
                "device": {
                    "id": 189760,
                    "name": "tor01-01"
                }
            }
        },
        "display": "tor01-01--eth0.0--forced--172.16.0.0/12",
        "created": "2026-10-07T13:20:16.085380Z",
        "last_updated": "2026-10-07T13:20:16.085389Z",
        "kind": "forced",
        "prefix": "172.16.0.0/12",
        "next_hop": null,
        "state": "staging",
        "monitoring_state": "disabled",
        "description": null
    },
    {
        "id": 3,
        "logical_interface": {
            "id": 1400,
            "index": 0,
            "name": "eth0.0",
            "parent_interface": {
                "id": 1300,
                "name": "eth0",
                "device": {
                    "id": 189761,
                    "name": "spine01-01"
                }
            }
        },
        "display": "spine01-01--eth0.0--forced--192.168.0.0/16",
        "created": "2026-10-07T13:20:16.085380Z",
        "last_updated": "2026-10-07T13:20:16.085389Z",
        "kind": "forced",
        "prefix": "192.168.0.0/16",
        "next_hop": null,
        "state": "staging",
        "monitoring_state": "disabled",
        "description": "by hand"
    }
]
`

func newTestManagementRoute(device string, index uint32, kind string, prefix types.CIDR, nextHop *net.IP) *interfaces.ManagementRoute {
	route := interfaces.ManagementRoute{Kind: kind, Prefix: prefix, NextHop: nextHop}
	route.LogicalInterface.Index = &index
	route.LogicalInterface.ParentInterface.Name = "eth0"
	route.LogicalInterface.ParentInterface.Device.Name = device
	return &route
}

func TestPrecomputeManagementRoutes(t *testing.T) {
	gateway := net.ParseIP("10.0.0.1")

	want := map[string][]*interfaces.ManagementRoute{
		"tor01-01": {
			newTestManagementRoute("tor01-01", 0, "static", types.CIDR{IP: net.ParseIP("0.0.0.0"), Netmask: 0}, &gateway),
			newTestManagementRoute("tor01-01", 0, "forced", types.CIDR{IP: net.ParseIP("172.16.0.0"), Netmask: 12}, nil),
		},
		"spine01-01": {
			newTestManagementRoute("spine01-01", 0, "forced", types.CIDR{IP: net.ParseIP("192.168.0.0"), Netmask: 16}, nil),
		},
	}

	var cmdbOutput []*interfaces.ManagementRoute
	if err := json.Unmarshal([]byte(managementRoutesJSON), &cmdbOutput); err != nil {
		t.Fatalf("unable to load test data: %s", err)
	}

	out := cmdb.PrecomputeManagementRoutes(cmdbOutput)
	if diff := cmp.Diff(out, want); diff != "" {
		t.Errorf("unexpected diff: %s\n", diff)
	}

	route := out["tor01-01"][0]
	if route.InterfaceName() != "eth0" || route.SubinterfaceIndex() != 0 {
		t.Errorf("unexpected logical interface: %s.%d", route.InterfaceName(), route.SubinterfaceIndex())
	}
}
