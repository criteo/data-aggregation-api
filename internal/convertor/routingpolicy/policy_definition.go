package routingpolicy

import (
	"errors"
	"strconv"
	"strings"

	"github.com/criteo/data-aggregation-api/internal/model/cmdb/routingpolicy"
	"github.com/criteo/data-aggregation-api/internal/model/openconfig"
)

const stringListSeparator = " "

var sourceProtocolMap = map[routingpolicy.RoutingProtocols]openconfig.E_PolicyTypes_INSTALL_PROTOCOL_TYPE{
	routingpolicy.Static:    openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
	routingpolicy.ISIS:      openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
	routingpolicy.OSPF:      openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_OSPF,
	routingpolicy.Connected: openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED,
	routingpolicy.BGP:       openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
	routingpolicy.Unset:     openconfig.PolicyTypes_INSTALL_PROTOCOL_TYPE_UNSET,
}

var decisionPolicyMap = map[routingpolicy.Decision]openconfig.E_RoutingPolicy_PolicyResultType{
	routingpolicy.Permit: openconfig.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	routingpolicy.Deny:   openconfig.RoutingPolicy_PolicyResultType_REJECT_ROUTE,
}

var setRouteOriginMap = map[routingpolicy.RouteProtocolOrigin]openconfig.E_BgpTypes_BgpOriginAttrType{
	routingpolicy.OriginEGP:        openconfig.BgpTypes_BgpOriginAttrType_EGP,
	routingpolicy.OriginIGP:        openconfig.BgpTypes_BgpOriginAttrType_IGP,
	routingpolicy.OriginIncomplete: openconfig.BgpTypes_BgpOriginAttrType_INCOMPLETE,
	routingpolicy.OriginNone:       openconfig.BgpTypes_BgpOriginAttrType_UNSET,
}

// extractASPathPrepend parses AS Path Prepend ("asn1 asn1 asn1" to asn1 * repeat).
func extractASPathPrepend(term *routingpolicy.RoutePolicyTerm) (*openconfig.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetAsPathPrepend, error) {
	asPathPrepend := &openconfig.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetAsPathPrepend{}
	if term.SetASPathPrependASN != nil {
		if term.SetASPathPrependRepeat == 0 {
			return nil, errors.New("as_path_prepend_repeat must be > 0 if as_path_prepend_asn is defined")
		}

		asPathPrepend.Asn = term.SetASPathPrependASN.Number
		asPathPrepend.RepeatN = &term.SetASPathPrependRepeat
	}
	return asPathPrepend, nil
}

func extractCommunities(term *routingpolicy.RoutePolicyTerm) []openconfig.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetCommunity_Inline_Communities_Union {
	var communities []openconfig.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetCommunity_Inline_Communities_Union
	for community := range strings.SplitSeq(term.SetCommunity, stringListSeparator) {
		if community != "" {
			communities = append(communities, openconfig.UnionString(community))
		}
	}
	return communities
}

func getStatementActions(term *routingpolicy.RoutePolicyTerm) (*openconfig.RoutingPolicy_PolicyDefinition_Statement_Actions, error) {
	var err error
	actions := openconfig.RoutingPolicy_PolicyDefinition_Statement_Actions{
		BgpActions: &openconfig.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions{},
	}

	actions.PolicyResult = decisionPolicyMap[term.Decision]
	if actions.BgpActions.SetAsPathPrepend, err = extractASPathPrepend(term); err != nil {
		return nil, err
	}
	actions.BgpActions.SetRouteOrigin = setRouteOriginMap[term.SetOrigin]

	communities := extractCommunities(term)
	if len(communities) > 0 {
		actions.BgpActions.SetCommunity = &openconfig.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetCommunity{
			Method:  openconfig.SetCommunity_Method_INLINE,
			Options: openconfig.BgpPolicy_BgpSetCommunityOptionType_REPLACE,
			Inline: &openconfig.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetCommunity_Inline{
				Communities: communities,
			},
		}
	}

	if term.SetMetric > 0 {
		actions.BgpActions.SetMed = openconfig.UnionUint32(term.SetMetric)
	}

	if term.SetLocalPref > 0 {
		actions.BgpActions.SetLocalPref = &term.SetLocalPref
	}

	if term.SetNextHop != nil {
		nextHop := openconfig.UnionString(term.SetNextHop.String())
		actions.BgpActions.SetNextHop = &nextHop
	}

	return &actions, nil
}

func getStatementConditions(term *routingpolicy.RoutePolicyTerm) *openconfig.RoutingPolicy_PolicyDefinition_Statement_Conditions {
	conditions := openconfig.RoutingPolicy_PolicyDefinition_Statement_Conditions{
		// CallPolicy:        nil, // TODO: implement sub-route-map to CMDB model
		InstallProtocolEq: sourceProtocolMap[term.FromSourceProtocol],
		BgpConditions:     &openconfig.RoutingPolicy_PolicyDefinition_Statement_Conditions_BgpConditions{
			// TODO: implement MedEq in CMDB
			// TODO: implement OriginEq in CMDB
			// TODO: implement NextHopIn in CMDB
			// TODO: implement AfiSafiIn in CMDB
			// TODO: implement ExtCommunitySet in CMDB
			// TODO: implement LargeCommunity in CMDB + Openconfig
		},
	}

	if term.FromPrefixList != nil && term.FromPrefixList.Name != "" {
		conditions.MatchPrefixSet = &openconfig.RoutingPolicy_PolicyDefinition_Statement_Conditions_MatchPrefixSet{
			PrefixSet: &term.FromPrefixList.Name,
		}
	}

	if term.FromLocalPref > 0 {
		conditions.BgpConditions.LocalPrefEq = &term.FromLocalPref
	}
	if term.FromBGPCommunityList != nil && term.FromBGPCommunityList.Name != "" {
		conditions.BgpConditions.CommunitySet = &term.FromBGPCommunityList.Name
	}

	return &conditions
}

func extractPolicyStatements(terms []*routingpolicy.RoutePolicyTerm) (*openconfig.RoutingPolicy_PolicyDefinition_Statement_OrderedMap, error) {
	var statements openconfig.RoutingPolicy_PolicyDefinition_Statement_OrderedMap

	for _, term := range terms {
		name := strconv.Itoa(term.Sequence)
		actions, err := getStatementActions(term)
		if err != nil {
			return nil, err
		}
		statement := openconfig.RoutingPolicy_PolicyDefinition_Statement{
			Name:       &name,
			Conditions: getStatementConditions(term),
			Actions:    actions,
		}

		if err := statements.Append(&statement); err != nil {
			return nil, err
		}
	}

	return &statements, nil
}

// RoutePoliciesToOpenconfig converts precomputed CMDB data to OpenConfig.
// OpenConfig path: /routing-policy/defined-sets/prefix-sets/.
func RoutePoliciesToOpenconfig(routePolicies []*routingpolicy.RoutePolicy) (map[string]*openconfig.RoutingPolicy_PolicyDefinition, error) {
	var policyDefinitions = make(map[string]*openconfig.RoutingPolicy_PolicyDefinition)

	for _, routePolicy := range routePolicies {
		terms, err := extractPolicyStatements(routePolicy.Terms)
		if err != nil {
			return nil, err
		}
		policy := openconfig.RoutingPolicy_PolicyDefinition{
			Name:      &routePolicy.Name,
			Statement: terms,
		}

		policyDefinitions[*policy.Name] = &policy
	}

	return policyDefinitions, nil
}
