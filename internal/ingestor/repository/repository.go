package repository

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/ingestor/cmdb"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/bgp"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/ntp"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/routingpolicy"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/snmp"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/sonic"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/syslog"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/tacacs"
	"github.com/criteo/data-aggregation-api/internal/model/dcim"
	"github.com/criteo/data-aggregation-api/internal/report"
)

func statsReport(message string, severity report.Severity) report.Message {
	return report.Message{
		Type:     report.IngestorMessage,
		Severity: severity,
		Text:     message,
	}
}

type AssetsPerDevice struct {
	BGPGlobal      map[string]*bgp.BGPGlobal
	BGPsessions    map[string][]*bgp.Session
	PeerGroups     map[string][]*bgp.PeerGroup
	PrefixLists    map[string][]*routingpolicy.PrefixList
	CommunityLists map[string][]*routingpolicy.CommunityList
	RoutePolicies  map[string][]*routingpolicy.RoutePolicy
	SNMP           map[string]*snmp.SNMP
	NTP            map[string]*ntp.NTP
	Syslog         map[string]*syslog.Syslog
	Tacacs         map[string]*tacacs.Tacacs
	// SONiCHwsku and SONiCType are keyed by DCIM device type and role ID.
	SONiCHwsku map[int]string
	SONiCType  map[int]string
}

type Assets struct {
	DeviceInventory    []*dcim.NetworkDevice
	CmdbBGPGlobal      []*bgp.BGPGlobal
	CmdbBGPSessions    []*bgp.Session
	CmdbPeerGroups     []*bgp.PeerGroup
	CmdbRoutePolicies  []*routingpolicy.RoutePolicy
	CmdbPrefixLists    []*routingpolicy.PrefixList
	CmdbCommunityLists []*routingpolicy.CommunityList
	CmdbSNMP           []*snmp.SNMP
	CmdbNTP            []*ntp.NTP
	CmdbSyslog         []*syslog.Syslog
	CmdbTacacs         []*tacacs.Tacacs
	CmdbSONiCHwsku     []*sonic.HwskuMapping
	CmdbSONiCRoles     []*sonic.RoleMapping
}

func (i *Assets) Precompute() *AssetsPerDevice {
	var precomputed AssetsPerDevice
	precomputed.BGPGlobal = cmdb.PrecomputeBGPGlobal(i.CmdbBGPGlobal)
	precomputed.BGPsessions = cmdb.PrecomputeBGPSessions(i.CmdbBGPSessions)
	precomputed.PeerGroups = cmdb.PrecomputePeerGroups(i.CmdbPeerGroups) //nolint:staticcheck // to ignore deprecation notice
	precomputed.PrefixLists = cmdb.PrecomputePrefixLists(i.CmdbPrefixLists)
	precomputed.CommunityLists = cmdb.PrecomputeCommunityLists(i.CmdbCommunityLists)
	precomputed.RoutePolicies = cmdb.PrecomputeRoutePolicies(i.CmdbRoutePolicies)
	precomputed.SNMP = cmdb.PrecomputeSNMP(i.CmdbSNMP)
	precomputed.NTP = cmdb.PrecomputeNTP(i.CmdbNTP)
	precomputed.Syslog = cmdb.PrecomputeSyslog(i.CmdbSyslog)
	precomputed.Tacacs = cmdb.PrecomputeTacacs(i.CmdbTacacs)
	precomputed.SONiCHwsku = cmdb.PrecomputeSONiCHwsku(i.CmdbSONiCHwsku)
	precomputed.SONiCType = cmdb.PrecomputeSONiCType(i.CmdbSONiCRoles)
	return &precomputed
}

func (i *Assets) getStats() map[string]int {
	return map[string]int{
		"devices":        len(i.DeviceInventory),
		"bgpGlobal":      len(i.CmdbBGPGlobal),
		"bgpSessions":    len(i.CmdbBGPSessions),
		"peerGroups":     len(i.CmdbPeerGroups),
		"routePolicies":  len(i.CmdbRoutePolicies),
		"prefixLists":    len(i.CmdbPrefixLists),
		"communityLists": len(i.CmdbCommunityLists),
		"SNMP":           len(i.CmdbSNMP),
		"NTP":            len(i.CmdbNTP),
		"syslog":         len(i.CmdbSyslog),
		"TACACS":         len(i.CmdbTacacs),
		"SONiCHwsku":     len(i.CmdbSONiCHwsku),
		"SONiCRoles":     len(i.CmdbSONiCRoles),
	}
}

// PrintStats prints number of asset per ingestor.
func (i *Assets) PrintStats() {
	for stat, val := range i.getStats() {
		log.Info().Str("stats", stat).Int("value", val).Msg("new assets found")
	}
}

// ReportStats sends stats to current Report.
func (i *Assets) ReportStats(messageChan chan<- report.Message) {
	for stat, val := range i.getStats() {
		messageChan <- statsReport(fmt.Sprintf("found %d %s", val, stat), report.Info)
	}
}
