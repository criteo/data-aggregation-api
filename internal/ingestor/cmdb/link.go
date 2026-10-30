package cmdb

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/ingestor/netbox"
	"github.com/criteo/data-aggregation-api/internal/model/cmdb/interfaces"
)

// GetLinks returns the links of the datacenter from the Network CMDB: the
// links with at least one end in the datacenter.
func GetLinks() ([]*interfaces.Link, error) {
	response := netbox.NetboxResponse[interfaces.Link]{}
	params := deviceDatacenterFilter()

	err := netbox.Get("/api/plugins/cmdb/links/", &response, params)
	if err != nil {
		return nil, fmt.Errorf("links fetching failure: %w", err)
	}

	if response.Count != len(response.Results) {
		log.Warn().Msg("some links have not been fetched")
	}

	return response.Results, nil
}

// PrecomputeNeighbors indexes each link from both ends: every device gets the
// remote end of each of its links, keyed by its local interface name.
//
// portLayouts gives the port layout of a device by hostname. It names the
// neighbor port the way the remote device names it (see PortLayoutTable.PortName).
func PrecomputeNeighbors(links []*interfaces.Link, portLayouts map[string]interfaces.PortLayoutTable) map[string]interfaces.NeighborTable {
	var neighborsPerDevice = make(map[string]interfaces.NeighborTable)

	addNeighbor := func(local, remote *interfaces.LinkEndpoint) {
		table, ok := neighborsPerDevice[local.Device.Name]
		if !ok {
			table = make(interfaces.NeighborTable)
			neighborsPerDevice[local.Device.Name] = table
		}

		// an interface in several links has no single neighbor: it is kept
		// as a nil entry so that later links do not reintroduce one
		if previous, duplicate := table[local.Name]; duplicate {
			if previous != nil {
				log.Warn().Msgf("interface %s of %s is in several links (to %s:%s and %s:%s), neighbor ignored",
					local.Name, local.Device.Name, previous.Device, previous.Port, remote.Device.Name, remote.Name)
				table[local.Name] = nil
			}
			return
		}

		table[local.Name] = &interfaces.Neighbor{
			Device:   remote.Device.Name,
			Port:     portLayouts[remote.Device.Name].PortName(remote.Name),
			CmdbPort: remote.Name,
		}
	}

	for _, link := range links {
		addNeighbor(&link.InterfaceA, &link.InterfaceB)
		addNeighbor(&link.InterfaceB, &link.InterfaceA)
	}

	return neighborsPerDevice
}
