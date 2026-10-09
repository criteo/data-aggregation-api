package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	bgpconvertors "github.com/criteo/data-aggregation-api/internal/convertor/bgp"
	ntpconvertors "github.com/criteo/data-aggregation-api/internal/convertor/ntp"
	rpconvertors "github.com/criteo/data-aggregation-api/internal/convertor/routingpolicy"
	snmpconvertors "github.com/criteo/data-aggregation-api/internal/convertor/snmp"
	syslogconvertors "github.com/criteo/data-aggregation-api/internal/convertor/syslog"
	tacacsconvertors "github.com/criteo/data-aggregation-api/internal/convertor/tacacs"
	"github.com/criteo/data-aggregation-api/internal/ingestor/repository"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/bgp"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/ntp"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/routingpolicy"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/snmp"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/syslog"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/tacacs"
	"github.com/criteo/data-aggregation-api/internal/model/dcim"
	"github.com/criteo/data-aggregation-api/internal/model/ietf"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
	"github.com/openconfig/ygot/ygot"
	"github.com/rs/zerolog/log"
)

const AFKEnabledTag = "afk-enabled"

var defaultInstance = "default"

type GeneratedConfig struct {
	IETF           *ietf.Device
	JSONIETF       string
	Openconfig     *openconfig.Device
	JSONOpenConfig string
}

type Device struct {
	mutex           *sync.Mutex
	Dcim            *dcim.NetworkDevice
	Config          *GeneratedConfig
	BGPGlobalConfig *bgp.BGPGlobal
	SNMP            *snmp.SNMP
	NTP             *ntp.NTP
	Syslog          *syslog.Syslog
	Tacacs          *tacacs.Tacacs
	Sessions        []*bgp.Session
	PeerGroups      []*bgp.PeerGroup
	PrefixLists     []*routingpolicy.PrefixList
	CommunityLists  []*routingpolicy.CommunityList
	RoutePolicies   []*routingpolicy.RoutePolicy
	AFKEnabled      bool

	// Hwsku and SONiCType are DEVICE_METADATA's hwsku and type, empty when
	// the device's model or role is not mapped.
	Hwsku     string
	SONiCType string
}

// isAFKenabled checks if the device contains the AFKEnabledTag.
func isAFKenabled(dcimInfo *dcim.NetworkDevice) bool {
	for _, tag := range dcimInfo.Tags {
		if tag.Name == AFKEnabledTag {
			return true
		}
	}
	return false
}

// NewDevice creates and populates a device with precomputed Ingestor's data.
func NewDevice(dcimInfo *dcim.NetworkDevice, devicesData *repository.AssetsPerDevice) (*Device, error) {
	device := &Device{
		mutex:      &sync.Mutex{},
		Dcim:       dcimInfo,
		AFKEnabled: isAFKenabled(dcimInfo),
	}

	// TODO: be able to set which ingestors are mandatory from the settings.yaml
	var ok bool

	// Check if there is CMDB data for the device
	device.Sessions, ok = devicesData.BGPsessions[dcimInfo.Hostname]
	if !ok {
		return nil, fmt.Errorf("no BGP session found for %s", dcimInfo.Hostname)
	}

	device.BGPGlobalConfig = devicesData.BGPGlobal[dcimInfo.Hostname]
	// TODO: uncomment once mandatory
	// if !ok {
	// 	return nil, fmt.Errorf("no BGP global configuration found for %s", dcimInfo.Hostname)
	// }

	device.PeerGroups, ok = devicesData.PeerGroups[dcimInfo.Hostname]
	if !ok {
		log.Warn().Msgf("no peer-groups found for %s", dcimInfo.Hostname)
	}

	device.PrefixLists, ok = devicesData.PrefixLists[dcimInfo.Hostname]
	if !ok {
		return nil, fmt.Errorf("no prefix-lists found for %s", dcimInfo.Hostname)
	}

	device.CommunityLists, ok = devicesData.CommunityLists[dcimInfo.Hostname]
	if !ok {
		return nil, fmt.Errorf("no community-lists found for %s", dcimInfo.Hostname)
	}

	device.RoutePolicies, ok = devicesData.RoutePolicies[dcimInfo.Hostname]
	if !ok {
		return nil, fmt.Errorf("no route-policies found for %s", dcimInfo.Hostname)
	}

	device.SNMP, ok = devicesData.SNMP[dcimInfo.Hostname]
	if !ok {
		log.Warn().Msgf("no snmp found for %s", dcimInfo.Hostname)
	}

	device.NTP, ok = devicesData.NTP[dcimInfo.Hostname]
	if !ok {
		log.Warn().Msgf("no ntp found for %s", dcimInfo.Hostname)
	}

	device.Syslog, ok = devicesData.Syslog[dcimInfo.Hostname]
	if !ok {
		log.Warn().Msgf("no syslog found for %s", dcimInfo.Hostname)
	}

	device.Tacacs, ok = devicesData.Tacacs[dcimInfo.Hostname]
	if !ok {
		log.Warn().Msgf("no tacacs found for %s", dcimInfo.Hostname)
	}

	device.Hwsku, ok = devicesData.SONiCHwsku[dcimInfo.DeviceType.ID]
	if !ok {
		log.Warn().Msgf("no SONiC HwSKU mapped to the device type %q of %s", dcimInfo.DeviceType.Model, dcimInfo.Hostname)
	}

	device.SONiCType, ok = devicesData.SONiCType[dcimInfo.DeviceRole.ID]
	if !ok {
		log.Warn().Msgf("no SONiC type mapped to the device role %q of %s", dcimInfo.DeviceRole.Name, dcimInfo.Hostname)
	}

	return device, nil
}

// Generateconfigs generate the Config (openconfig & ietf) data for the current device.
// The CMDB data must have been precomputed before running this method.
func (d *Device) Generateconfigs() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Generate sub-configs
	bgpConfig, err := bgpconvertors.BGPToOpenconfig(d.Dcim.Hostname, d.BGPGlobalConfig, d.Sessions, d.PeerGroups)
	if err != nil {
		return fmt.Errorf("convert from BGP to OpenConfig failed: %w", err)
	}

	routingPolicyConfig, err := rpconvertors.RoutingPolicyToOpenconfig(d.PrefixLists, d.CommunityLists, d.RoutePolicies)
	if err != nil {
		return fmt.Errorf("convert from Routing Policy to OpenConfig failed: %w", err)
	}

	aaaConfig, err := tacacsconvertors.TacacsToOpenConfigAAA(d.Tacacs)
	if err != nil {
		return fmt.Errorf("convert from TACACS to OpenConfig failed: %w", err)
	}

	// Assemble global configuration
	bgpKey := openconfig.NetworkInstance_Protocol_Key{Identifier: openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, Name: "bgp"}

	config := openconfig.Device{
		RoutingPolicy: routingPolicyConfig,
		NetworkInstance: map[string]*openconfig.NetworkInstance{
			"default": {
				Name: &defaultInstance,
				Protocol: map[openconfig.NetworkInstance_Protocol_Key]*openconfig.NetworkInstance_Protocol{
					bgpKey: {
						Bgp:        bgpConfig,
						Name:       &bgpKey.Name,
						Identifier: openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
					},
				},
			},
		},
		System: &openconfig.System{
			Hostname: optional(d.Dcim.Hostname),
			Hwsku:    optional(d.Hwsku),
			Type:     optional(d.SONiCType),
			Aaa:      aaaConfig,
			Ntp:      ntpconvertors.NTPToOpenconfig(d.NTP),
			Logging:  syslogconvertors.SyslogToOpenconfig(d.Syslog),
		},
	}

	devJSON, err := ygot.EmitJSON(
		&config,
		&ygot.EmitJSONConfig{
			Format:         ygot.RFC7951,
			SkipValidation: false,
			Indent:         "  ",
		},
	)

	if err != nil {
		return fmt.Errorf("failed to transform an openconfig device specification (%s) into JSON using ygot: %w", d.Dcim.Hostname, err)
	}

	// IETF configuration
	d.Config = &GeneratedConfig{
		Openconfig:     &config,
		JSONOpenConfig: devJSON,
		IETF:           nil,
		JSONIETF:       "{}",
	}

	if d.SNMP == nil {
		log.Warn().Msgf("%s don't have a Snmp configuration, skip IetfConfig", d.Dcim.Hostname)
	} else {
		IetfSystem := snmpconvertors.SNMPtoIETFfSystem(d.SNMP)
		IetfSnmp := snmpconvertors.SNMPtoIETFsnmp(d.SNMP)

		d.Config.IETF = &ietf.Device{
			System: &IetfSystem,
			Snmp:   &IetfSnmp,
		}

		d.Config.JSONIETF, err = ygot.EmitJSON(
			d.Config.IETF,
			&ygot.EmitJSONConfig{
				Format:         ygot.RFC7951,
				SkipValidation: false,
				Indent:         "  ",
			},
		)
		if err != nil {
			return fmt.Errorf("failed to transform an ietf device specification (%s) into JSON using ygot: %w", d.Dcim.Hostname, err)
		}
	}

	return nil
}

// optional returns a pointer to s, or nil when s is empty so that the leaf is
// left out rather than emitted empty.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetCompactOpenconfigJSON returns OpenConfig result in not indented JSON format.
// Generated JSON is already indented by Ygot - currently there is no option to not indent the JSON.
func (d *Device) GetCompactOpenconfigJSON() ([]byte, error) {
	out := bytes.NewBuffer(nil)
	err := json.Compact(out, []byte(d.Config.JSONOpenConfig))
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// GetCompactIETFJSON returns IETF result in not indented JSON format.
// GetCompactIETFJSON JSON is already indented by Ygot - currently there is no option to not indent the JSON.
func (d *Device) GetCompactIETFJSON() ([]byte, error) {
	out := bytes.NewBuffer(nil)
	err := json.Compact(out, []byte(d.Config.JSONIETF))
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
