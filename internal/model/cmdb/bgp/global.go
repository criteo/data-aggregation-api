package bgp

import "github.com/criteo/data-aggregation-api/internal/model/cmdb/common"

type Network struct {
	ID     *int   `json:"id,omitempty"`
	Prefix string `json:"prefix" validate:"required"`
}

type GlobalAfiSafi struct {
	ID                    *int          `json:"id,omitempty"`
	Name                  AfiSafiChoice `json:"afi_safi_name"             validate:"required"`
	Aggregates            []Network     `json:"aggregates"                validate:"required"`
	RedistributedNetworks []Network     `json:"redistributed_networks"    validate:"required"`
}

type BGPGlobal struct {
	Device struct {
		Name string `json:"name" validate:"required"`
	} `json:"device" validate:"required"`
	LocalAsn common.ASN `json:"local_asn"                    validate:"required"`

	EBGPAdministrativeDistance *uint8           `json:"ebgp_administrative_distance" validate:"omitempty"`
	IBGPAdministrativeDistance *uint8           `json:"ibgp_administrative_distance" validate:"omitempty"`
	GracefulRestartEnabled     *bool            `json:"graceful_restart"             validate:"required"`
	GracefulRestartTime        *uint16          `json:"graceful_restart_time"        validate:"omitempty"`
	EcmpEnabled                *bool            `json:"ecmp"                         validate:"required"`
	EcmpMaximumPaths           *uint32          `json:"ecmp_maximum_paths"           validate:"omitempty"`
	RouterID                   string           `json:"router_id"                    validate:"omitempty"`
	AfiSafis                   []*GlobalAfiSafi `json:"afi_safis"                    validate:"omitempty"`
}
