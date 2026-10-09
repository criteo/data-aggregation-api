package cmdb_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/criteo/data-aggregation-api/internal/ingestor/cmdb"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/sonic"
)

func TestPrecomputeSONiCHwsku(t *testing.T) {
	// As the netbox-cmdb plugin returns them: the device type is nested.
	const response = `[
	  {
	    "id": 1,
	    "device_type": {"id": 10, "display": "MSN2700", "manufacturer": {"id": 1, "name": "Mellanox"}, "model": "MSN2700", "slug": "msn2700"},
	    "hwsku": "ACS-MSN2700",
	    "created": "2026-10-08T07:37:29Z", "last_updated": "2026-10-08T07:37:29Z"
	  },
	  {
	    "id": 2,
	    "device_type": {"id": 11, "display": "MSN4700", "model": "MSN4700", "slug": "msn4700"},
	    "hwsku": "ACS-MSN4700"
	  }
	]`

	var mappings []*sonic.HwskuMapping
	if err := json.Unmarshal([]byte(response), &mappings); err != nil {
		t.Fatal(err)
	}

	want := map[int]string{10: "ACS-MSN2700", 11: "ACS-MSN4700"}
	if diff := cmp.Diff(cmdb.PrecomputeSONiCHwsku(mappings), want); diff != "" {
		t.Errorf("unexpected diff (-got +want):\n%s", diff)
	}
}

func TestPrecomputeSONiCType(t *testing.T) {
	const response = `[
	  {"id": 1, "device_role": {"id": 20, "name": "ToR", "slug": "tor"}, "sonic_type": "ToRRouter"},
	  {"id": 2, "device_role": {"id": 21, "name": "Spine", "slug": "spine"}, "sonic_type": "SpineRouter"},
	  {"id": 3, "device_role": {"id": 22, "name": "Lab", "slug": "lab"}, "sonic_type": "not-provisioned"}
	]`

	var mappings []*sonic.RoleMapping
	if err := json.Unmarshal([]byte(response), &mappings); err != nil {
		t.Fatal(err)
	}

	// not-provisioned is no SONiC type: the role is left unmapped.
	want := map[int]string{20: "ToRRouter", 21: "SpineRouter"}
	if diff := cmp.Diff(cmdb.PrecomputeSONiCType(mappings), want); diff != "" {
		t.Errorf("unexpected diff (-got +want):\n%s", diff)
	}
}
