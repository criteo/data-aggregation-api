package cmdb_test

import (
	"encoding/json"
	"testing"

	"github.com/criteo/data-aggregation-api/internal/ingestor/cmdb"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/dns"
	"github.com/google/go-cmp/cmp"
)

// dnsConfig is a DNS configuration as Netbox returns it, Device being an anonymous struct.
func dnsConfig(hostname string, servers ...dns.Server) *dns.DNS {
	config := &dns.DNS{ServerList: servers}
	config.Device.Name = hostname
	return config
}

func TestPrecomputeDNS(t *testing.T) {
	tests := []struct {
		name string
		args string
		want map[string]*dns.DNS
	}{
		{
			name: "devices sharing servers",
			args: `[
         {
            "id":1,
            "device":{"id":1, "name":"tor01-01"},
            "server_list":[
               {"id":1, "name":"dns1", "server_address":"192.0.2.53", "created":"2026-10-22T07:37:29Z", "last_updated":"2026-10-22T07:37:29Z"},
               {"id":2, "name":"dns2", "server_address":"2001:db8::53", "created":"2026-10-22T07:37:29Z", "last_updated":"2026-10-22T07:37:29Z"}
            ],
            "created":"2026-10-22T07:37:29Z",
            "last_updated":"2026-10-22T07:37:29Z"
         },
         {
            "id":2,
            "device":{"id":2, "name":"tor01-02"},
            "server_list":[
               {"id":1, "name":"dns1", "server_address":"192.0.2.53", "created":"2026-10-22T07:37:29Z", "last_updated":"2026-10-22T07:37:29Z"}
            ],
            "created":"2026-10-22T07:37:29Z",
            "last_updated":"2026-10-22T07:37:29Z"
         }
      ]`,
			want: map[string]*dns.DNS{
				"tor01-01": dnsConfig("tor01-01",
					dns.Server{Name: "dns1", ServerAddress: "192.0.2.53"},
					dns.Server{Name: "dns2", ServerAddress: "2001:db8::53"},
				),
				"tor01-02": dnsConfig("tor01-02",
					dns.Server{Name: "dns1", ServerAddress: "192.0.2.53"},
				),
			},
		},
	}

	for _, test := range tests {
		var cmdbOutput []*dns.DNS
		if err := json.Unmarshal([]byte(test.args), &cmdbOutput); err != nil {
			t.Errorf("unable to load test data for '%s': %s", test.name, err)
			continue
		}

		out := cmdb.PrecomputeDNS(cmdbOutput)
		if diff := cmp.Diff(out, test.want); diff != "" {
			t.Errorf("unexpected diff for '%s': %s\n", test.name, diff)
		}
	}
}
