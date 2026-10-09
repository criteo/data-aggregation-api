package device

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/criteo/data-aggregation-api/internal/ingestor/repository"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/bgp"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/ntp"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/routingpolicy"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/tacacs"
	"github.com/criteo/data-aggregation-api/internal/model/dcim"
)

// TestGenerateconfigsSystem checks that the NTP and the TACACS+ configurations,
// which both live under openconfig-system, are served side by side, next to
// system/config which always holds the hostname.
func TestGenerateconfigsSystem(t *testing.T) {
	tests := []struct {
		name     string
		ntp      *ntp.NTP
		tacacs   *tacacs.Tacacs
		wantKeys []string
	}{
		{
			name:     "NTP and TACACS+",
			ntp:      &ntp.NTP{ServerList: []ntp.Server{{Name: "ntp1", ServerAddress: "10.0.0.1"}}},
			tacacs:   &tacacs.Tacacs{Passkey: "secret", ServerList: []tacacs.Server{{ServerAddress: "10.10.10.10", Priority: 1, TCPPort: 49}}},
			wantKeys: []string{"aaa", "config", "ntp"},
		},
		{
			name:     "NTP only",
			ntp:      &ntp.NTP{ServerList: []ntp.Server{{Name: "ntp1", ServerAddress: "10.0.0.1"}}},
			wantKeys: []string{"config", "ntp"},
		},
		{
			name:     "TACACS+ only",
			tacacs:   &tacacs.Tacacs{ServerList: []tacacs.Server{{ServerAddress: "10.10.10.10"}}},
			wantKeys: []string{"aaa", "config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Device{
				mutex:           &sync.Mutex{},
				Dcim:            &dcim.NetworkDevice{Hostname: "test-device"},
				BGPGlobalConfig: &bgp.BGPGlobal{},
				NTP:             tt.ntp,
				Tacacs:          tt.tacacs,
			}

			if err := d.Generateconfigs(); err != nil {
				t.Fatalf("Generateconfigs() failed: %s", err)
			}

			var got struct {
				System map[string]json.RawMessage `json:"system"`
			}
			if err := json.Unmarshal([]byte(d.Config.JSONOpenConfig), &got); err != nil {
				t.Fatalf("invalid generated JSON: %s", err)
			}

			system, _ := json.MarshalIndent(got.System, "", "  ")
			t.Logf("system:\n%s", system)

			if len(got.System) != len(tt.wantKeys) {
				t.Errorf("system has %d containers, want %v", len(got.System), tt.wantKeys)
			}
			for _, key := range tt.wantKeys {
				if _, ok := got.System[key]; !ok {
					t.Errorf("system.%s is missing from the output", key)
				}
			}
		})
	}
}

const (
	deviceTypeID = 10
	deviceRoleID = 20
)

func tor() *dcim.NetworkDevice {
	d := &dcim.NetworkDevice{Hostname: "tor01-01"}
	d.DeviceType.ID, d.DeviceType.Model = deviceTypeID, "MSN2700"
	d.DeviceRole.ID, d.DeviceRole.Name = deviceRoleID, "ToR"
	return d
}

// assets holds everything NewDevice requires for tor01-01, mappings included.
func assets() *repository.AssetsPerDevice {
	return &repository.AssetsPerDevice{
		BGPsessions:    map[string][]*bgp.Session{"tor01-01": {}},
		PrefixLists:    map[string][]*routingpolicy.PrefixList{"tor01-01": {}},
		CommunityLists: map[string][]*routingpolicy.CommunityList{"tor01-01": {}},
		RoutePolicies:  map[string][]*routingpolicy.RoutePolicy{"tor01-01": {}},
		SONiCHwsku:     map[int]string{deviceTypeID: "ACS-MSN2700"},
		SONiCType:      map[int]string{deviceRoleID: "ToRRouter"},
	}
}

func TestNewDevice_ResolvesTheSONiCMappings(t *testing.T) {
	dev, err := NewDevice(tor(), assets())
	if err != nil {
		t.Fatal(err)
	}
	if dev.Hwsku != "ACS-MSN2700" || dev.SONiCType != "ToRRouter" {
		t.Errorf("Hwsku, SONiCType = %q, %q", dev.Hwsku, dev.SONiCType)
	}
}

// TestNewDevice_UnmappedIsOptional: a device without a mapped model or role
// is still built, without that DEVICE_METADATA fact.
func TestNewDevice_UnmappedIsOptional(t *testing.T) {
	data := assets()
	data.SONiCHwsku, data.SONiCType = nil, nil

	dev, err := NewDevice(tor(), data)
	if err != nil {
		t.Fatalf("NewDevice failed on an unmapped model and role: %v", err)
	}
	if dev.Hwsku != "" || dev.SONiCType != "" {
		t.Errorf("Hwsku, SONiCType = %q, %q, want empty", dev.Hwsku, dev.SONiCType)
	}
}

// generateSystem runs Generateconfigs on tor01-01 holding only the system
// data and returns the emitted JSON.
func generateSystem(hwsku, sonicType string, ntpConfig *ntp.NTP) (string, error) {
	d := &Device{
		mutex:           &sync.Mutex{},
		Dcim:            &dcim.NetworkDevice{Hostname: "tor01-01"},
		BGPGlobalConfig: &bgp.BGPGlobal{},
		NTP:             ntpConfig,
		Hwsku:           hwsku,
		SONiCType:       sonicType,
	}
	if err := d.Generateconfigs(); err != nil {
		return "", err
	}
	return d.Config.JSONOpenConfig, nil
}

// systemConfig returns system/config of an emitted device.
func systemConfig(t *testing.T, raw string) map[string]any {
	t.Helper()
	var doc struct {
		System struct {
			Config map[string]any `json:"config"`
		} `json:"system"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return doc.System.Config
}

// TestGenerateconfigsDeviceMetadata pins the contract with afk-node: the
// augmented leaves are emitted under system/config with bare names, next to
// hostname.
func TestGenerateconfigsDeviceMetadata(t *testing.T) {
	raw, err := generateSystem("ACS-MSN2700", "ToRRouter", nil)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{"hostname": "tor01-01", "hwsku": "ACS-MSN2700", "type": "ToRRouter"}
	if diff := cmp.Diff(systemConfig(t, raw), want); diff != "" {
		t.Errorf("unexpected system/config (-got +want):\n%s\nfull JSON:\n%s", diff, raw)
	}
}

// TestGenerateconfigsDeviceMetadata_Unmapped: a device without mappings still
// gets its hostname, and no empty hwsku or type.
func TestGenerateconfigsDeviceMetadata_Unmapped(t *testing.T) {
	raw, err := generateSystem("", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{"hostname": "tor01-01"}
	if diff := cmp.Diff(systemConfig(t, raw), want); diff != "" {
		t.Errorf("unexpected system/config (-got +want):\n%s", diff)
	}
}

// TestGenerateconfigsDeviceMetadata_TypeOutsideThePattern: the YANG pattern is
// enforced, so a type afk-node does not support never reaches it.
func TestGenerateconfigsDeviceMetadata_TypeOutsideThePattern(t *testing.T) {
	for _, sonicType := range []string{"BackEndToRRouter", "not-provisioned"} {
		if _, err := generateSystem("ACS-MSN2700", sonicType, nil); err == nil {
			t.Errorf("type %q was emitted, want a validation error", sonicType)
		}
	}
}

// TestGenerateconfigsNTP: the servers land where afk-node reads them,
// system/ntp/servers/server[].address.
func TestGenerateconfigsNTP(t *testing.T) {
	config := &ntp.NTP{ServerList: []ntp.Server{
		{Name: "ntp1", ServerAddress: "192.0.2.1"},
		{Name: "ntp2", ServerAddress: "192.0.2.2"},
	}}
	raw, err := generateSystem("ACS-MSN2700", "ToRRouter", config)
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		System struct {
			NTP struct {
				Servers struct {
					Server []struct {
						Address string `json:"address"`
					} `json:"server"`
				} `json:"servers"`
			} `json:"ntp"`
		} `json:"system"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(doc.System.NTP.Servers.Server))
	for _, s := range doc.System.NTP.Servers.Server {
		got = append(got, s.Address)
	}
	if diff := cmp.Diff(got, []string{"192.0.2.1", "192.0.2.2"}); diff != "" {
		t.Errorf("unexpected NTP servers (-got +want):\n%s\nfull JSON:\n%s", diff, raw)
	}
}
