package cmdb_test

import (
	"encoding/json"
	"testing"

	"github.com/criteo/data-aggregation-api/internal/ingestor/cmdb"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
	"github.com/google/go-cmp/cmp"
)

const linksJSON = `
[
    {
        "id": 3895,
        "interface_a": {
            "id": 5256,
            "name": "to_spine_01",
            "device": {"id": 191479, "name": "tor01-01"}
        },
        "interface_b": {
            "id": 5257,
            "name": "to_leaf_01",
            "device": {"id": 191474, "name": "spine01-01"}
        },
        "state": "staging",
        "monitoring_state": "disabled",
        "display": "tor01-01--to_spine_01 <--> spine01-01--to_leaf_01"
    },
    {
        "id": 3896,
        "interface_a": {
            "id": 5258,
            "name": "eth0",
            "device": {"id": 191479, "name": "tor01-01"}
        },
        "interface_b": {
            "id": 5259,
            "name": "to_tor01-01",
            "device": {"id": 191480, "name": "oob01-01"}
        },
        "state": "staging",
        "monitoring_state": "disabled"
    }
]
`

func loadLinks(t *testing.T) []*interfaces.Link {
	t.Helper()
	var links []*interfaces.Link
	if err := json.Unmarshal([]byte(linksJSON), &links); err != nil {
		t.Fatalf("unable to load test data: %s", err)
	}
	return links
}

func TestPrecomputeNeighbors(t *testing.T) {
	spinePort := &interfaces.PortLayout{Name: "etp1", LogicalName: "to_leaf_01", VendorName: "Ethernet0"}
	portLayouts := map[string]interfaces.PortLayoutTable{
		"spine01-01": {"to_leaf_01": spinePort},
	}

	want := map[string]interfaces.NeighborTable{
		"tor01-01": {
			"to_spine_01": {Device: "spine01-01", Interface: "to_leaf_01", Port: spinePort},
			"eth0":        {Device: "oob01-01", Interface: "to_tor01-01"},
		},
		"spine01-01": {
			"to_leaf_01": {Device: "tor01-01", Interface: "to_spine_01"},
		},
		"oob01-01": {
			"to_tor01-01": {Device: "tor01-01", Interface: "eth0"},
		},
	}

	out := cmdb.PrecomputeNeighbors(loadLinks(t), portLayouts)
	if diff := cmp.Diff(out, want); diff != "" {
		t.Errorf("unexpected diff: %s\n", diff)
	}
}

func TestPrecomputeNeighborsSeveralLinks(t *testing.T) {
	links := loadLinks(t)
	third := *links[0]
	third.InterfaceB.Name = "to_leaf_99"
	links = append(links, &third, &third)

	out := cmdb.PrecomputeNeighbors(links, nil)

	neighbor, ok := out["tor01-01"]["to_spine_01"]
	if !ok || neighbor != nil {
		t.Errorf("an interface in several links must have no neighbor, got %+v", neighbor)
	}
	if out["tor01-01"]["eth0"] == nil {
		t.Error("other interfaces must keep their neighbor")
	}
}

func TestPrecomputeNeighborsEmpty(t *testing.T) {
	out := cmdb.PrecomputeNeighbors(nil, nil)
	if len(out) != 0 {
		t.Errorf("expected empty result, got %d entries", len(out))
	}
}
