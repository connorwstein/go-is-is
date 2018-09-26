package main

import (
	"bytes"
	"flag"
	"github.com/vishvananda/netlink"
	"net"
	"testing"
)

func setDebugs(verbosity string) {
	// Debugging failed testcase utility
	flag.Set("alsologtostderr", "true")
	flag.Set("v", verbosity)
	flag.Parse()
}

func TestDecisionSPF3Node(t *testing.T) {
	// TOPO:  R1 -- 10 -- R2 -- 10 -- R3
	// Result should be:
	// R1 0  nexthop nil
	// R2 10 nexthop R2 eth0
	// R3 20 nexthop R2 eth0

	// Build a sample update database then apply SPF
	// Required for this: interfaces with routes and adjacencies to build the reachability and neighbor TLVs
	// Adjacencies need a neighbor system id
	// TODO: maybe this topology can be extracted automatically from a docker-compose file which could also be used for scale tests
	// will need a way to generate scale topologies eventually
	initConfig()
	updateDBInit()

	// R1
	r1Interfaces := make([]*Intf, 1)
	r1Interfaces[0] = &Intf{adj: &Adjacency{metric: 10, state: "UP", neighborSystemID: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x12}, intfName: "eth0"}}
	r1Interfaces[0].routes = make([]*net.IPNet, 1)
	r1Interfaces[0].routes[0] = &net.IPNet{IP: net.IP{172, 20, 0, 0}, Mask: net.IPMask{0xff, 0xff, 0, 0}}
	r1sid := "1111.1111.1111"
	r1neighborTLV := getNeighborTLV(r1Interfaces)
	r1reachTLV := getIPReachTLV(r1Interfaces)
	r1lsp := buildEmptyLSP(1, r1sid)
	r1reachTLV.nextTLV = r1neighborTLV
	r1lsp.CoreLsp.FirstTLV = r1reachTLV
	UpdateDB.Root = AvlInsert(UpdateDB.Root, systemIDToKey(r1sid), r1lsp, false)

	// R2
	r2Interfaces := make([]*Intf, 2)
	r2Interfaces[0] = &Intf{adj: &Adjacency{metric: 10, state: "UP", neighborSystemID: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11}, intfName: "eth0"}}
	r2Interfaces[1] = &Intf{adj: &Adjacency{metric: 10, state: "UP", neighborSystemID: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x13}, intfName: "eth1"}}
	r2Interfaces[0].routes = make([]*net.IPNet, 1)
	r2Interfaces[1].routes = make([]*net.IPNet, 1)
	r2Interfaces[0].routes[0] = &net.IPNet{IP: net.IP{172, 20, 0, 0}, Mask: net.IPMask{0xff, 0xff, 0, 0}}
	r2Interfaces[1].routes[0] = &net.IPNet{IP: net.IP{172, 19, 0, 0}, Mask: net.IPMask{0xff, 0xff, 0, 0}}
	r2sid := "1111.1111.1112"
	r2neighborTLV := getNeighborTLV(r2Interfaces)
	r2reachTLV := getIPReachTLV(r2Interfaces)
	r2lsp := buildEmptyLSP(1, r2sid)
	r2reachTLV.nextTLV = r2neighborTLV
	r2lsp.CoreLsp.FirstTLV = r2reachTLV
	UpdateDB.Root = AvlInsert(UpdateDB.Root, systemIDToKey(r2sid), r2lsp, false)

	// R3
	r3Interfaces := make([]*Intf, 1)
	r3Interfaces[0] = &Intf{adj: &Adjacency{metric: 10, state: "UP", neighborSystemID: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x12}, intfName: "eth0"}}
	r3Interfaces[0].routes = make([]*net.IPNet, 1)
	r3Interfaces[0].routes[0] = &net.IPNet{IP: net.IP{172, 19, 0, 0}, Mask: net.IPMask{0xff, 0xff, 0, 0}}
	r3sid := "1111.1111.1113"
	r3neighborTLV := getNeighborTLV(r3Interfaces)
	r3reachTLV := getIPReachTLV(r3Interfaces)
	r3lsp := buildEmptyLSP(1, r3sid)
	r3reachTLV.nextTLV = r3neighborTLV
	r3lsp.CoreLsp.FirstTLV = r3reachTLV
	UpdateDB.Root = AvlInsert(UpdateDB.Root, systemIDToKey(r3sid), r3lsp, false)

	printUpdateDB(UpdateDB.Root)

	// Lets compute SPF from the perspective of R1, R2 and R3
	//turnFlagsOn()
	var Topo1 *IsisDB = &IsisDB{}
	var Topo2 *IsisDB = &IsisDB{}
	var Topo3 *IsisDB = &IsisDB{}
	computeSPF(UpdateDB, Topo1, r1sid, r1Interfaces)
	computeSPF(UpdateDB, Topo2, r2sid, r2Interfaces)
	computeSPF(UpdateDB, Topo3, r3sid, r3Interfaces)
	// Inspect the topology learned by each node
	topo1 := AvlGetAll(Topo1.Root)
	for _, node := range topo1 {
		trip := node.data.(*Triple)
		if (trip.systemID == r1sid && trip.distance != 0) ||
			(trip.systemID == r2sid && trip.distance != 10) ||
			(trip.systemID == r3sid && trip.distance != 20) {
			t.Fail()
		}
	}
	topo2 := AvlGetAll(Topo2.Root)
	for _, node := range topo2 {
		trip := node.data.(*Triple)
		if (trip.systemID == r1sid && trip.distance != 10) ||
			(trip.systemID == r2sid && trip.distance != 0) ||
			(trip.systemID == r3sid && trip.distance != 10) {
			t.Fail()
		}
	}
	topo3 := AvlGetAll(Topo3.Root)
	for _, node := range topo3 {
		trip := node.data.(*Triple)
		if (trip.systemID == r1sid && trip.distance != 20) ||
			(trip.systemID == r2sid && trip.distance != 10) ||
			(trip.systemID == r3sid && trip.distance != 0) {
			t.Fail()
		}
	}
}

func TestInstallRoute(t *testing.T) {
	initConfig()
	// Lookup and LSP from the database
	testSystemID := "1111.1111.1111"
	// Neighbor IP is the next hop
	// Needs to be a realistic next hop or linux wont like it use 172.18.0.100
	trip := Triple{systemID: testSystemID, adj: &Adjacency{neighborIP: net.ParseIP("172.18.0.100")}}
	// Can't use the real routes
	routes := make([]*net.IPNet, 0)
	routes = append(routes, &net.IPNet{IP: net.ParseIP("172.28.0.0").To4(), Mask: []byte{0xff, 0xff, 0, 0}})
	t.Logf("Routes %v", routes)
	cfg.interfaces = append(cfg.interfaces, &Intf{routes: routes})
	reachTLV := getIPReachTLV(cfg.interfaces)
	t.Logf("Reach TLV %v", reachTLV)
	lsp := IsisLsp{CoreLsp: &IsisLspCore{FirstTLV: reachTLV}}
	updateDBInit()
	UpdateDB.Root = AvlInsert(UpdateDB.Root, systemIDToKey(testSystemID), &lsp, false)
	testRoute := netlink.Route{Dst: &net.IPNet{IP: net.ParseIP("172.28.0.0").To4(), Mask: []byte{0xff, 0xff, 0, 0}}, Gw: net.ParseIP("172.18.0.100")}
	netlink.RouteDel(&testRoute)
	installRouteFromPath(&trip)
	// Check whether those routes actually get installed
	routesInstalled, _ := netlink.RouteList(nil, 0)
	t.Logf("Installed routes %v", routesInstalled)
	found := false
	for _, route := range routesInstalled {
		if bytes.Equal(route.Gw.To4(), testRoute.Gw.To4()) {
			found = true
		}
	}
	if !found {
		t.Fail()
	}
	netlink.RouteDel(&testRoute)
}
