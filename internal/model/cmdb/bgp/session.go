package bgp

import (
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/common"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/routingpolicy"
	"github.com/criteo/data-aggregation-api/internal/types"
)

type Address struct {
	Address types.CIDR `json:"address" validate:"required"`
	Family  int        `json:"family"  validate:"required"`
}

type DeviceSession struct {
	Device struct {
		Name string `json:"name" validate:"required"`
	} `json:"device" validate:"required"`

	LocalAsn        common.ASN                     `json:"local_asn"        validate:"required"`
	PeerGroup       *PeerGroupLite                 `json:"peer_group"       validate:"omitempty"`
	RoutePolicyIn   *routingpolicy.RoutePolicyLite `json:"route_policy_in"  validate:"omitempty"`
	RoutePolicyOut  *routingpolicy.RoutePolicyLite `json:"route_policy_out" validate:"omitempty"`
	AfiSafis        []*AfiSafi                     `json:"afi_safis"        validate:"required"`
	Enabled         *bool                          `json:"enabled"          validate:"required"`
	Description     string                         `json:"description"      validate:"omitempty"`
	LocalAddress    Address                        `json:"local_address"    validate:"required"`
	MaximumPrefixes uint32                         `json:"maximum_prefixes" validate:"omitempty"`
	DelayOpenTimer  uint16                         `json:"delay_open_timer" validate:"omitempty"`
	EnforceFirstAs  bool                           `json:"enforce_first_as" validate:"omitempty"`
}

type Session struct {
	Password string        `json:"password" validate:"omitempty"`
	PeerA    DeviceSession `json:"peer_a"   validate:"required"`
	PeerB    DeviceSession `json:"peer_b"   validate:"required"`
}
