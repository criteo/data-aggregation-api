package repository

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/ingestor/cmdb"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/bgp"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/dhcprelay"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/dns"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
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
	BGPGlobal         map[string]*bgp.BGPGlobal
	BGPsessions       map[string][]*bgp.Session
	PeerGroups        map[string][]*bgp.PeerGroup
	PrefixLists       map[string][]*routingpolicy.PrefixList
	CommunityLists    map[string][]*routingpolicy.CommunityList
	RoutePolicies     map[string][]*routingpolicy.RoutePolicy
	SNMP              map[string]*snmp.SNMP
	NTP               map[string]*ntp.NTP
	DNS               map[string]*dns.DNS
	Syslog            map[string]*syslog.Syslog
	Tacacs            map[string]*tacacs.Tacacs
	DHCPRelays        map[string][]*dhcprelay.Relay
	DeviceInterfaces  map[string][]*interfaces.DeviceInterface
	LogicalInterfaces map[string][]*interfaces.LogicalInterface
	ManagementRoutes  map[string][]*interfaces.ManagementRoute
	// PortLayouts are not per device but per hardware model and network role.
	PortLayouts map[interfaces.PortLayoutKey]interfaces.PortLayoutTable
	Neighbors   map[string]interfaces.NeighborTable
	// SONiCHwsku and SONiCType are keyed by DCIM device type and role ID.
	SONiCHwsku map[int]string
	SONiCType  map[int]string
}

type Assets struct {
	DeviceInventory       []*dcim.NetworkDevice
	CmdbBGPGlobal         []*bgp.BGPGlobal
	CmdbBGPSessions       []*bgp.Session
	CmdbPeerGroups        []*bgp.PeerGroup
	CmdbRoutePolicies     []*routingpolicy.RoutePolicy
	CmdbPrefixLists       []*routingpolicy.PrefixList
	CmdbCommunityLists    []*routingpolicy.CommunityList
	CmdbSNMP              []*snmp.SNMP
	CmdbNTP               []*ntp.NTP
	CmdbDNS               []*dns.DNS
	CmdbSyslog            []*syslog.Syslog
	CmdbTacacs            []*tacacs.Tacacs
	CmdbDHCPRelays        []*dhcprelay.Relay
	CmdbSONiCHwsku        []*sonic.HwskuMapping
	CmdbSONiCRoles        []*sonic.RoleMapping
	CmdbDeviceInterfaces  []*interfaces.DeviceInterface
	CmdbLogicalInterfaces []*interfaces.LogicalInterface
	CmdbPortLayouts       []*interfaces.PortLayout
	CmdbLinks             []*interfaces.Link
	CmdbManagementRoutes  []*interfaces.ManagementRoute
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
	precomputed.DeviceInterfaces = cmdb.PrecomputeDeviceInterfaces(i.CmdbDeviceInterfaces)
	precomputed.LogicalInterfaces = cmdb.PrecomputeLogicalInterfaces(i.CmdbLogicalInterfaces)
	precomputed.ManagementRoutes = cmdb.PrecomputeManagementRoutes(i.CmdbManagementRoutes)
	precomputed.PortLayouts = cmdb.PrecomputePortLayouts(i.CmdbPortLayouts)
	precomputed.Neighbors = cmdb.PrecomputeNeighbors(i.CmdbLinks, i.portLayoutPerDevice(precomputed.PortLayouts))
	precomputed.NTP = cmdb.PrecomputeNTP(i.CmdbNTP)
	precomputed.DNS = cmdb.PrecomputeDNS(i.CmdbDNS)
	precomputed.Syslog = cmdb.PrecomputeSyslog(i.CmdbSyslog)
	precomputed.Tacacs = cmdb.PrecomputeTacacs(i.CmdbTacacs)
	precomputed.DHCPRelays = cmdb.PrecomputeDHCPRelays(i.CmdbDHCPRelays)
	precomputed.SONiCHwsku = cmdb.PrecomputeSONiCHwsku(i.CmdbSONiCHwsku)
	precomputed.SONiCType = cmdb.PrecomputeSONiCType(i.CmdbSONiCRoles)
	return &precomputed
}

// portLayoutPerDevice resolves the port layout of every device of the
// inventory. Devices outside the inventory (e.g. in another datacenter) or
// without a port layout are absent.
func (i *Assets) portLayoutPerDevice(portLayouts map[interfaces.PortLayoutKey]interfaces.PortLayoutTable) map[string]interfaces.PortLayoutTable {
	layouts := make(map[string]interfaces.PortLayoutTable, len(i.DeviceInventory))
	for _, device := range i.DeviceInventory {
		key := interfaces.PortLayoutKey{DeviceTypeID: device.DeviceType.ID, RoleID: device.Role.ID}
		if layout, ok := portLayouts[key]; ok {
			layouts[device.Hostname] = layout
		}
	}
	return layouts
}

func (i *Assets) getStats() map[string]int {
	return map[string]int{
		"devices":           len(i.DeviceInventory),
		"bgpGlobal":         len(i.CmdbBGPGlobal),
		"bgpSessions":       len(i.CmdbBGPSessions),
		"peerGroups":        len(i.CmdbPeerGroups),
		"routePolicies":     len(i.CmdbRoutePolicies),
		"prefixLists":       len(i.CmdbPrefixLists),
		"communityLists":    len(i.CmdbCommunityLists),
		"SNMP":              len(i.CmdbSNMP),
		"NTP":               len(i.CmdbNTP),
		"DNS":               len(i.CmdbDNS),
		"syslog":            len(i.CmdbSyslog),
		"TACACS":            len(i.CmdbTacacs),
		"DHCPRelays":        len(i.CmdbDHCPRelays),
		"SONiCHwsku":        len(i.CmdbSONiCHwsku),
		"SONiCRoles":        len(i.CmdbSONiCRoles),
		"deviceInterfaces":  len(i.CmdbDeviceInterfaces),
		"logicalInterfaces": len(i.CmdbLogicalInterfaces),
		"portLayouts":       len(i.CmdbPortLayouts),
		"links":             len(i.CmdbLinks),
		"managementRoutes":  len(i.CmdbManagementRoutes),
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
