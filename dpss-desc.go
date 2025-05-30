// package desc contains functions for creating descriptions of mrnes
// objects that are used at initialization.  These can be called from
// code that creates them or reads them up from file.  Serialization
// functions here can also write created ones to file
package desc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
)


// var IdValue holds a counter used to create id numbers
var IdValue int = 1

// NextId gets 'the next' sequential id from a counter that
// is monotonically increasing, ensuring that assigned node
// numbers are unique
func NextId() int {
	id := IdValue
	IdValue += 1
	return id
}

// numberOfIntrfcs (and more generally, numberOf{Objects}
// are counters of the number of default instances of each
// object type have been created, and so can be used
// to help create unique default names for these objects
var numberOfIntrfcs int = 0
var numberOfNetworks int = 0
var numberOfRouters int = 0
var numberOfEndpoints int = 0

// defaultIntrfcBndwdth is one of several default
// values that are used if the user model does not
// specify them.  Mostly these values relate
// to speeds and capacities of network components
//    Bandwidth and capacity units are MBytes/sec
//    Time values are seconds
var defaultIntrfcBndwdth float64 = 100.0
var defaultNetworkIntrfcBndwdth float64 = 500.0
var defaultBackhaulIntrfcBndwdth float64 = 2000.0
var defaultNetworkPrefixLen = 12
var defaultNetworkLatency float64 = 1e-3
var defaultEndpointRate float64 = 100000.0
var defaultLANCapacity float64 = 10000.0
var defaultWANCapacity float64 = 10000.0
var defaultT3Capacity float64 = 10000.0
var defaultT2Capacity float64 = 100000.0
var defaultT1Capacity float64 = 1000000.0

// GlobalpgnDescTypes gives a slice of pre-defined ProtocolNode types. These
// appear in the slice in an order such that a type cannot be connected
// in a protocol graph in a node as a parent of a node with a type that
// appears later in the slice.  The types are inspired by the standard
// ISO network stack model
var GlobalpgnDescTypes = []string{"nic", "network", "endpoint", "transport", "delay", "rateflow", "congestion", "sim", "server", "offline", "online"}

// to avoid the creation of loops in the protocol graph we impose the rule that a node c cannot be the parent
// of another node p if c's ordering index is less than p's.  What we need is a partial order
var PgnTypeOrder = map[string]int{"nic": 0, "network": 1, "endpoint": 2, "transport": 3, "delay":3, "rateflow": 3, "congestion": 6,
	"sim": 6, "server": 6, "offline": 6, "online": 6}

// DefaultCapacityByLevel helps to convert between integer-enumeration-based expression of
// the total capacity of a network component (by component type) to a string
// that also encodes it.
func DefaultCapacityByLevel(level string) float64 {
	switch level {
	case "Endpoint":
		return defaultEndpointRate
	case "LAN":
		return defaultLANCapacity
	case "WAN":
		return defaultWANCapacity
	case "T3":
		return defaultT3Capacity
	case "T2":
		return defaultT2Capacity
	case "T1":
		return defaultT1Capacity
	default:
		return defaultWANCapacity
	}
}

// IsConnectable checks for an assumed rule that network connectivity 
// is governed by network level.   Two networks that are not equal or
// adjacent in the hierarchy

func IsConnectable(level_1, level_2 string) bool {
	if level_1 == level_2 {
		return true
	}
	// put in alphabetical order to cut down on comparisons
	if level_2 < level_1 {
		tmp := level_2
		level_2 = level_1
		level_1 = tmp
	}
	if level_1 == "Endpoint" && level_2 == "LAN" {
		return true
	}

	if level_1 == "LAN" && level_2 == "WAN" {
		return true
	}

	if level_1 == "T3" && level_2 == "WAN" {
		return true
	}

	if level_1 == "T2" && level_2 == "T3" {
		return true
	}

	if level_1 == "T1" && level_2 == "T2" {
		return true
	}
	return false
}

// DefaultCapacityByLevel helps to convert between integer-enumeration-based expression of
// the total bandwidth of a network component's interface (by component type) to a string
// that also encodes it.
func DefaultIntrfcBndwdthByLevel(level string) float64 {
	switch level {
	case "Endpoint":
		return defaultIntrfcBndwdth
	case "LAN":
		return defaultNetworkIntrfcBndwdth
	case "WAN":
		return defaultNetworkIntrfcBndwdth
	case "T3":
		return defaultBackhaulIntrfcBndwdth
	case "T2":
		return defaultBackhaulIntrfcBndwdth
	case "T1":
		return defaultBackhaulIntrfcBndwdth
	default:
		return defaultNetworkIntrfcBndwdth
	}
}

// DefaultIntrfcName automatically crafts a string name for an interface,
// mostly for reporting and identification purposes.  Will be unique
// among calls to the function
func DefaultIntrfcName() string {
	numberOfIntrfcs += 1
	name := "Interface_" + strconv.Itoa(numberOfIntrfcs)
	return name
}

// DefaultNetworkName automatically crafts a string name for a network,
// mostly for reporting and identification purposes.  Will be unique
// among calls to the function
func DefaultNetworkName() string {
	numberOfNetworks += 1
	name := "Network_" + strconv.Itoa(numberOfNetworks)
	return name
}

// DefaultRouterName automatically crafts a string name for a router,
// mostly for reporting and identification purposes.  Will be unique
// among calls to the function
func defaultRouterName() string {
	numberOfRouters += 1
	name := "Router_" + strconv.Itoa(numberOfRouters)
	return name
}

// DefaultEndpointName automatically crafts a string name for an endpoint (e.g., host)
// mostly for reporting and identification purposes.  Will be unique
// among calls to the function
func DefaultEndpointName() string {
	numberOfEndpoints += 1
	name := "Endpoint_" + strconv.Itoa(numberOfEndpoints)
	return name
}

// DefaultEndpointRelativeCPU assigns a default value of relative CPU speed
// This is not directly tied to cycle times.  A relative speed of 2 
// is twice as fast as a relative speed of 1.  An command line parameter
// of workPerMusec  indicates how much 'work' is accomplished on the CPU
// per micro-second of computing on a CPU with relative speed 1
// 
func DefaultEndpointRelativeCPU() float64 {
	return 1.0
}

// DefaultEndpointMemory assigns a default value of available memory, measured in GB
// 
func DefaultEndpointMemory () float64 {
	return 10.0
}


// SetdefaultIntrfcBndwdth and in general SetFault{NetworkType}Bndwdth
// gives a user the ability to dynamically change the default values
// baked into the code.  They are _still_ defaults, used when other
// values aren't specified, but they give the user some control
// over the default assignments
func SetdefaultIntrfcBndwdth(default_bndwdth float64) {
	defaultIntrfcBndwdth = default_bndwdth
}

func SetdefaultNetworkIntrfcBndwdth(default_bndwdth float64) {
	defaultNetworkIntrfcBndwdth = default_bndwdth
}

func SetdefaultBackhaulIntrfcBndwdth(default_bndwdth float64) {
	defaultBackhaulIntrfcBndwdth = default_bndwdth
}

func SetdefaultLANCapacity(default_capacity float64) {
	defaultLANCapacity = default_capacity
}

func SetdefaultWANCapacity(default_capacity float64) {
	defaultWANCapacity = default_capacity
}

func SetdefaultT3Capacity(default_capacity float64) {
	defaultT3Capacity = default_capacity
}

func SetdefaultT2Capacity(default_capacity float64) {
	defaultT2Capacity = default_capacity
}

func SetdefaultT1Capacity(default_capacity float64) {
	defaultT1Capacity = default_capacity
}

func SetdefaultNetworkLatency(default_latency float64) {
	defaultNetworkLatency = default_latency
}

func SetdefaultNetworkPrefixLen(default_prefixLen int) {
	defaultNetworkPrefixLen = default_prefixLen
}

func SetdefaultEndpointRate(default_rate float64) {
	defaultEndpointRate = default_rate
}

// IntrfcDesc describes parameters of a network interface, except for bandwidth which needs to be FINDME
type IntrfcDesc struct {
	Bandwidth float64
	Faces     string
	Number    int
	IP_Addr   string
}

// Networkdesc describes parameters of a Network in the topology
type NetworkDesc struct {
	Level     string
	Name      string
	NetAddr   string
	Number    int
	PrefixLen int
	Latency   float64
	Capacity  float64
	Ext_conn  []string
}

// RouterDesc describes parameters of a Router in the topology
type RouterDesc struct {
	Name       string
	Number     int
	Interfaces []IntrfcDesc
}

// EndpointDesc describes parameters of an Endpoint in the topology
type EndpointDesc struct {
	External   bool
	Name       string
	PrefixLen  int
	Number     int
	RelativeCPUSpeed  float64
	Memory		float64
	Interfaces []IntrfcDesc
}

// func CreateIntrfcDesc creates a [IntrfcDesc] from argment parameters
// Bandwidth units are Mbps, Faces is the text Name of a Network the interface touches,
// IP_Addr is a particular built-in representation for IP addresses whose use
// should allow either IPV4 or IPV6
//
//	Attributes bndwdth and IP_Addr are pointers to allow us to easily test for non-presence
//
// Arguments 'bndwdth' and IP_Addr are both pointers, with the understanding
// that if the pointer is nil the constructor should use a default
func CreateIntrfcDesc(bndwdth *float64, faces string, IP_Addr *netip.Addr,
	network_by_name map[string]*NetworkDesc) (*IntrfcDesc, error) {

	// extract a string presentation of the IP address
	var ip_addr string = ""
	if IP_Addr != nil {
		ip_addr = fmt.Sprintf("%s", *IP_Addr)
	}

	network, present := network_by_name[faces]
	if !present {
		err := errors.New(fmt.Sprintf("network named as being faced (%s) does not exist", faces))
		return nil, err
	}

	// set the bandwidth to the default if needed.
	// Particular default value depends on the level of the network
	default_bndwdth := DefaultIntrfcBndwdthByLevel(network.Level)
	if bndwdth == nil {
		// default is needed
		bndwdth = &default_bndwdth
	}
	return &IntrfcDesc{Bandwidth: *bndwdth, Faces: faces, Number: NextId(), IP_Addr: ip_addr}, nil
}

// CreateNetworkDesc builds a [NetworkDesc] from its input parameters
//
// Level" is a string from {Endpoint, LAN, WAN, T3, T2, T1} describing
//
//	the kind of network.   There are constraints on the level's of networks
//	that can connect, e.g. a WAN can only connect down to LANs and up to T3s
//
// Arguments of CreateNetworkDesc: "name" is any text string desired, no assumptions made about it,
// "prefix" is a pointer to an object representing a network.  For an IPv4 network
// it is the prefix length, e.g. 12 for 10.12.8.0/12.   If
//
//		prefix is empty the "prefixLen" will define the size of the network, it is otherwise ignored.
//		If not empty we get the prefix value from NetAddr.
//	   "latency" has units of seconds and gives the length of time a bit takes to traverse
//	 the network when there is no congestion.
//		"capacity" describes the total capacity for the network to carry traffic from an ingress
//		point to an egress point, in units of Mbps.  CreateNetworkDesc creates a slice Ex_conn
//	 for names of other network components to which this one directly connects,
//		but leaves it empty to be filled through connection calls
func CreateNetworkDesc(level string, name string, prefix *netip.Prefix, prefixLen int,
	latency *float64, capacity *float64, network_by_name map[string]*NetworkDesc) (*NetworkDesc, error) {

	// create an instance and fill in its attributes from the function arguments
	nd := new(NetworkDesc)
	nd.Level = level

	// The Id is created arbitrarily and automatically
	nd.Number = NextId()

	// either read in a name or create one automatically
	if len(name) > 0 {
		nd.Name = name
	} else {
		nd.Name = DefaultNetworkName()
	}

	// save in the mapping between network name and the network description object
	network_by_name[nd.Name] = nd

	// initialize the NetAddr and PrefixLen attributes depending on whether
	// a non-empty pointer to a netip.Prefix is an argument
	if prefix != nil {
		nd.NetAddr = fmt.Sprintf("%s", *prefix)
		nd.PrefixLen = (*prefix).Bits()
	} else {
		nd.NetAddr = ""
		nd.PrefixLen = prefixLen
	}

	// If latency is not specified use a default value
	if latency != nil {
		nd.Latency = *latency
	} else {
		nd.Latency = defaultNetworkLatency
	}

	// If end-to-end capacity is not specified, use a default value
	if capacity != nil {
		nd.Capacity = *capacity
	} else {
		nd.Capacity = DefaultCapacityByLevel(level)
	}

	// initialize the slice of external connections
	nd.Ext_conn = make([]string, 0)

	// rigorously an error return is not needed, but one can imagine
	// that a future modification could generate one and we'd want to
	// report it
	return nd, nil
}

// func CreateRouterDesc creates a [RouterDesc] from input parameters.
// Name is a unique string identifier, Number is a unique integer identifier,
// and Interfaces is a list of json-formatted description of interfaces such
// as having been created by func CreateIntrfcDesc
func CreateRouterDesc(name string, intrfc_descs *[]IntrfcDesc,
	network_by_name map[string]*NetworkDesc) (*RouterDesc, error) {

	rd := new(RouterDesc)

	// give the router an id number automatically
	rd.Number = NextId()

	// either read in a name or create one automatically
	if len(name) > 0 {
		rd.Name = name
	} else {
		rd.Name = defaultRouterName()
	}

	err_slice := []error{}

	// if a slice of interface descriptions is given
	if intrfc_descs != nil {
		rd.Interfaces = *intrfc_descs

		// gather a mapping between names of networks that are identified in the
		// interfaces as being faced and their description objects (for us
		edgeNetworks := make(map[string]*NetworkDesc)

		for _, intrfc_desc := range rd.Interfaces {
			edgeNetworks[intrfc_desc.Faces] = network_by_name[intrfc_desc.Faces]
		}

		// ensure that every pair of networks connecting to the router
		// are seen in their Ext_conn lists as connecting to each other,
		// provided that their network levels are equal or adjacent
		//
		for name1, net1 := range edgeNetworks {
			for name2, net2 := range edgeNetworks {

				// do the subsequent analysis only for every unique pair of networks
				if name2 <= name1 {
					continue
				}

				// connections permitted only between networks of adjacent or same levels
				if !IsConnectable(net1.Level, net2.Level) {
					err := errors.New(fmt.Sprintf("route %s asked to connect level conflicting networks %s and %s", name, name1, name2))
					err_slice = append(err_slice, err)
					continue
				}

				// if name2 is not in the Ext_conn list of net1, put it there
				found := false
				for idx := 0; !found && idx < len(net1.Ext_conn); idx++ {
					found = (net1.Ext_conn[idx] == name2)
				}

				if !found {
					net1.Ext_conn = append(net1.Ext_conn, name2)
				}

				// if name1 is not in the Ext_conn list of net2, put it there
				found = false
				for idx := 0; !found && idx < len(net2.Ext_conn); idx++ {
					found = (net2.Ext_conn[idx] == name1)
				}

				if !found {
					net2.Ext_conn = append(net2.Ext_conn, name1)
				}
			}
		}
	} else {
		// no interfaces given so do initialization that will support our adding them later
		rd.Interfaces = make([]IntrfcDesc, 0)
	}
	var report_err error = nil
	if len(err_slice) > 0 {
		report_err = AggregateErrors(err_slice)
	}
	return rd, report_err
}

// func CreateEndpointDesc creates an [EndpointDesc] from input parameters
//
// a) If non-empty, processing_rate points to a float giving the speed
//
//	at which the endpoint can generate/receive traffic.
//
// b) external is true if the endpoint is for emulation or represents a real device
// c) name is a unique string identifier
// d) prefixLen describes the 'width' of the endpoint's IP presence, allow us to
//
//	introduce an endpoint to represent more than just one device, e.g.,
//	a space opened for a honeypot
//
// e) intrfc gives (if non-empty) a list of interfaces attached to the endpoint.
//
//	N.B. more than one is allowed, e.g. for multi-homed hosts
func CreateEndpointDesc(relativeCPUSpeed *float64, memoryCapacity *float64, external bool,
	name string, prefixLen int, intrfcs *[]IntrfcDesc) (*EndpointDesc, error) {

	endpt := new(EndpointDesc)

	// we're told whether the endpoint is internal or external
	endpt.External = external

	if relativeCPUSpeed != nil {
		endpt.RelativeCPUSpeed = *relativeCPUSpeed
	} else {
		endpt.RelativeCPUSpeed = DefaultEndpointRelativeCPU()
	}

	if memoryCapacity != nil {
		endpt.Memory = *memoryCapacity 
	} else {
		endpt.Memory = DefaultEndpointMemory()
	}

	// get a (presumeably unique) string name
	if len(name) > 0 {
		endpt.Name = name
	} else {
		endpt.Name = DefaultEndpointName()
	}

	// get a unique integer identity
	endpt.Number = NextId()

	// either copy in existing interfaces, or prepare to have interfaces attached
	// by other code
	if intrfcs != nil {
		endpt.Interfaces = *intrfcs
	} else {
		endpt.Interfaces = make([]IntrfcDesc, 0)
	}

	// error not needed now, but framework prepared for possible future inclusion as needed
	return endpt, nil
}

// func ConnectNetworks ensures that the named networks are represented in the topology as connecting
//
//	They may connect already connect through a given router, in which case we just ensure
//
// that the networks appear (if permitted) in each other's Ext_conn lists.
// Otherwise we we create a router to make the connection and create
// default interfaces for it
//
// a) name1 and name2 are the names given to the network descriptions when they were created
// b) rtr is either empty (implying that a router is to be created) or is the router to use in the connection
// c) network_by_name points to the global map we used to get a network description object, from knowing its name
//
// Return either the router passed in, or the router created for this connection
func ConnectNetworks(name1, name2 string, rtr *RouterDesc, network_by_name map[string]*NetworkDesc) (*RouterDesc, error) {
	err_slice := []error{}

	// make sure that the named networks actually exist
	_, present1 := network_by_name[name1]
	_, present2 := network_by_name[name2]
	if present1 || present2 {
		err := errors.New(fmt.Sprintf("network existence problem in ConnectNetworks"))
		return nil, err
	}

	if rtr == nil {
		// create a default router that connects name1 and name2

		// the router will have one interface facing name1, the other
		// facing name2.  These need to be created.  Use default interface
		//   appropriate to the level of the network being faced

		// first ensure that the networks referenced exist
		net1, present1 := network_by_name[name1]
		if !present1 {
			err := errors.New(fmt.Sprintf("ConnectNetworks cites non-extant network %s", name1))
			err_slice = append(err_slice, err)
		}
		net2, present2 := network_by_name[name2]
		if !present2 {
			err := errors.New(fmt.Sprintf("ConnectNetworks cites non-extant network %s", name2))
			err_slice = append(err_slice, err)
		}

		if len(err_slice) > 0 {
			err := AggregateErrors(err_slice)
			return nil, err
		}

		if !IsConnectable(net1.Level, net2.Level) {
			err := errors.New(fmt.Sprintf("ConnectNetworks cites level-separated networks %s, %s",
				name1, name2))
			return nil, err
		}

		// put name1 and name2 in each other's Ext_conn arrays
		found := false
		for _, conn := range net1.Ext_conn {
			if conn == name2 {
				found = true
				break
			}
		}
		if !found {
			net1.Ext_conn = append(net1.Ext_conn, name2)
		}

		found = false
		for _, conn := range net2.Ext_conn {
			if conn == name1 {
				found = true
				break
			}
		}
		if !found {
			net2.Ext_conn = append(net2.Ext_conn, name2)
		}

		// get default interface bandwidths as a function of the network level
		default_net1 := DefaultIntrfcBndwdthByLevel(net1.Level)
		default_net2 := DefaultIntrfcBndwdthByLevel(net2.Level)

		// no known IP addresses at this time. N.B. conditions that cause
		// CreateIntrfcDesc to throw an error have already been tested
		intrfc_desc1, _ := CreateIntrfcDesc(&default_net1, name1, nil, network_by_name)
		intrfc_desc2, _ := CreateIntrfcDesc(&default_net2, name2, nil, network_by_name)

		// set up an interface description slice for creating a router
		intrfc_desc_List := make([]IntrfcDesc, 0)
		intrfc_desc_List = append(intrfc_desc_List, *intrfc_desc1)
		intrfc_desc_List = append(intrfc_desc_List, *intrfc_desc2)

		// create a router. First make a unique default name
		rtrName := "default_" + name1 + "/" + name2
		if name2 < name1 {
			rtrName = "default_" + name2 + "/" + name1
		}

		// we're good to go
		new_rtr, err := CreateRouterDesc(rtrName, &intrfc_desc_List, network_by_name)

		if err != nil {
			return nil, err
		} else {
			return new_rtr, nil
		}
	}

	// we are given a router to use for connection
	// see the network names are already faced by some interface on the router
	routerFaces := make(map[string]bool)
	for _, intrfc_desc := range rtr.Interfaces {
		routerFaces[intrfc_desc.Faces] = true
	}

	_, present := routerFaces[name1]
	if !present {
		intrfc_desc1, _ := CreateIntrfcDesc(nil, name1, nil, network_by_name)
		rtr.Interfaces = append(rtr.Interfaces, *intrfc_desc1)
	}

	// do the same thing for name2
	_, present = routerFaces[name2]
	if !present {
		intrfc_desc2, _ := CreateIntrfcDesc(nil, name2, nil, network_by_name)
		rtr.Interfaces = append(rtr.Interfaces, *intrfc_desc2)
	}
	return rtr, nil
}

// type TopoDescFrame gives the highest level structure of the dictionary,
// Topology components types as attributes, lists of associated object
// descriptions as the values
//
//	The thing about the TopoDescFrame is that pointers to the
//
// various network object instances are saved.   For serialization
// we'll need to have actual memory-copied instances of those in similar
// lists, and there is a dedicated function CreateTopoDescDict()
// that does that
type TopoDescFrame struct {
	Endpoints []*EndpointDesc
	Networks  []*NetworkDesc
	Routers   []*RouterDesc
}

// CreateTopoDescFrame constructs an instance of a TopoDescFram
func CreateTopoDescFrame() TopoDescFrame {
	TF := new(TopoDescFrame)
	TF.Endpoints = make([]*EndpointDesc, 0)
	TF.Networks = make([]*NetworkDesc, 0)
	TF.Routers = make([]*RouterDesc, 0)
	return *TF
}

// AddEndpoint appends a pointer to a constructed EndpointDesc to the TopoDescFrame's list of EndpointDesc
func (tf *TopoDescFrame) AddEndpoint(endpt *EndpointDesc) {
	tf.Endpoints = append(tf.Endpoints, endpt)
}

// AddNetwork appends a pointer to a constructed RouterDesc to the TopoDescFrame's list of NetworkDesc
func (tf *TopoDescFrame) AddNetwork(net *NetworkDesc) {
	tf.Networks = append(tf.Networks, net)
}

// AddRouter appends a pointer to a constructed RouterDesc to the TopoDescFrame's list of RouterDesc
func (tf *TopoDescFrame) AddRouter(rtr *RouterDesc) {
	tf.Routers = append(tf.Routers, rtr)
}

// CreateTopoDescDict() transforms the slices of pointers to network objects
// into slices of instances of those objects
func (tf *TopoDescFrame) CreateTopoDescDict() Topodict {
	TD := new(Topodict)

	TD.Endpoints = make([]EndpointDesc, 0)
	for _, endpt := range tf.Endpoints {
		TD.Endpoints = append(TD.Endpoints, *endpt)
	}

	TD.Networks = make([]NetworkDesc, 0)
	for _, net := range tf.Networks {
		TD.Networks = append(TD.Networks, *net)
	}

	TD.Routers = make([]RouterDesc, 0)
	for _, rtr := range tf.Routers {
		TD.Routers = append(TD.Routers, *rtr)
	}

	return *TD
}

// Serialize calls the json Marshal function to create a json formated
// expression of the dictionary structure
func (tf *TopoDescFrame) Serialize() ([]byte, error) {
	TopoDescDict := tf.CreateTopoDescDict()
	bytes, merr := json.MarshalIndent(TopoDescDict, "", "\t")
	if merr != nil {
		return nil, merr
	} else {
		return bytes, nil
	}
}

// Serialize calls the json Marshal function to create a json formated
// expression of the dictionary structure
func (td *Topodict) Serialize() ([]byte, error) {
	bytes, merr := json.MarshalIndent(td, "", "\t")
	if merr != nil {
		return nil, merr
	} else {
		return bytes, nil
	}
}

// Protocol Graphs and Traffic
//
// Endpoints host 'protocol graphs' of functionality, based on the OSF Network stack concept.
// Abstractly, at an application layer, network traffic is generated and/received.
// We have different ways of doing this, and different functionalities.  These different ways are
// each represented by a different kind of 'ProtocolNode' (which defines a Go interface).
// A ProtocolNode may receive messages from one or more ProtocolNodes that are 'above' it,
// do some processing, and push messages down to one or more ProtocolNodes that are below it.
// Thus these connections
// define edges between instances of protocol nodes, and the whole thing is a graph.
//   An topological Endpoint is home to a graph and serves as next-to-last ProtocolNode if
// one imagines traffic from originating higher in the graph and being pushed down to the Endpoint
// layer and then a network.
//
// We use a struct called a SysEndptCfg struct.  In Go this is a structure with
// attributes.
//
//     type SysEndptCfg struct {
//            PatternMap map[string]Pattern
//            EndptCfgs map[string]EndptCfg
//      }
//
// The idea is that an Endpoint's configuration is defined by a Pattern of a
// protocol graph, e.g. a structure with given nodes, types, and between them,
// and a mapping of values.    For example, one structure might have one source of rate-based
// traffic and the system have many of these, another structure might represent
// the attachment of an emulation to the simulator, another might represent
// a live connection between external device and the simulator.   In the SysEndptCfg
// struct every unique Pattern has a name, and the collection of all Patterns found
// in the system is organized as a dictionary that uses the Pattern's name to index
// to a description of the Pattern.
//
// In Go a Pattern has the description
//
// Pattern describes a Graph of Protocol Node descriptions (PGNDesc)
//
//      type Pattern struct {
//          Name string
//	        PGNDescTypes []string
//	        PGNDescDict map[string]string
//	        GraphEdges []GraphEdge
//      }
//
// with
//
//  type PGNDesc struct {
//    	Name string
//    	Type string
//  }
//
// The PGNDesc identifies a graph node abstractly in terms of a 'Type' and a 'Name'.
// The Type is meant to convey the functional role
// of such a protocol node, e.g., "transport" to provide transport layer functionality.
// A superset of protocol graph node types is listed in the PGNDescTypes slice,
// which also conveys a sense of ordering, about which more, anon.

// A node's name may be abitrary except that all PGNDesc instances with a given
// Pattern must have unique names.  A Pattern may have multiple nodes with the same type
// (e.g. different Pcap trace sources that have the same source), so the Name provides
// unique referencing capability.  The same PGNDesc Name may be used by different Patterns.
//
// Pairs of protocol graph nodes may have a parent-child relationship, each one is expressed
// as a GraphEdge
//
//  type GraphEdge struct {
//  	ChildName   string
//  	ParentName  string
//  	ChildLabel  string
//  	ParentLabel string
//  }
//
// Here we see the role that attribute 'Name'  from PGNDesc plays, in identifying
// the graph nodes to be associated, and the nature of the relationship.
// A protocol graph is meant to be a directed acyclic graph, with a GraphEdge
// saying that the root of the edge is the Parent and the target of the edge is the
// child.  We ensure that the protocol graph described in a Pattern is acyclic using
// a partial order on PGNDesc types which the slice PGNDescs represents. If type
// type1 appears before type2 in PGNDesc, it is not permitted that a node of type type1
// be declared by a GraphEdge to be a Parent of a node with type type2.  If we enforce
// this constraint when creating graph edges we will be assured that the graph is acyclic.
//   It is not rigorously required that the protocol graph be connected, although a warning
// is generated if it is not.
//   In the code that actually makes these connections (and not just declare them, as here)
// the child node will be given ParentLabel to associate with the Parent node,
// and the parent node will be given ChildLabel to associate with the Child node.
// This will allow the code which creates information that a Parent passes to a Child
// to reference the Parent to receive the code by the label.
//
// Particular instances of protocol graphs for named Endpoints are gathered in the SysEndptCfg
// EndptCfgs map.  The index to the map is the Endpoint name given to the topology constructor
// when building the network.  These names are unique across the system, so specifying the
// name completely identifies the Endpoint.  The configuration to be applied to that endpoint
// is encoded in
//
//	type EndptCfg struct {
//	    EndpointName string
//	    PatternName  string
//	    ParameterMap map[string]map[string]string
//	}
//
//	The EndpointName is the same as the name used to index to this struct,
//  PatternName identifies one of the Patterns declared earlier
//	in SysEndptCfg.PatternMap, and ParameterMap holds information to be used
//  to configure the Endpoint.  The index to the PatternMap is a node Name
//  associated with some PGNDesc in the Pattern's PGNDesc slice.
//  so that the slice of ParameterDef structs are inputs to the
//  to the specifically named PGNDesc node from the specifically named Pattern
//  on the specifically named endpoint.   A ParameterDef is just an association
//	between some user-defined parameter (e.g. "Inter-arrival rate") and
//  description of a value to associate with that parameter.
//
//		type ParameterDef struct {
//		    Parameter string
//		    Value string
//		}
//
// structs (like Endpoint) that satisfy a particular Go interface (called ProtocolNode) definition
// for protocol graph nodes must have a function with signature
//
//		Initialize(map[string]string) (any, error)
//
// After the data structure representing an Endpoint and its protocol graph is constructed,
// the startup phase for the simulator finds all the instantiations of graph nodes
// that have lists of ParameterDef structs, and calls their Initialize functions passing in as
// an argument the slice described in an EndptCfg.
//

// PGNDesc names a protocol graph node description solely by a (unique to a Pattern) name,
// and a Type.  Every system has predefined types "network", "endpoint", "transport",
// a user can define others, but must also define supporting funcs that describe the
// type's behavior.
type PGNDesc struct {
	Name string
	Type string
}

type PGNDescMap map[string]string

// Print is used when debugging protocol graph development
func (pgn *PGNDesc) Print() {
	fmt.Printf("PGNDesc name %s, type %s\n", pgn.Name, pgn.Type)
}

// PGNDescTypePresent determines for a given Pattern whether a string-valued type
// is associated with the Pattern (e.g. appears in its PGNDescTypes slice)
func (ptn *PatternFrame) PGNDescTypePresent(node_type string) bool {

	// PGNDescTypeOrder is a lookup table to avoid linear scanes of PGNDescTypes
	_, present := ptn.PGNDescTypeOrder[node_type]
	return present
}

// CreatePGNDesc takes the parameters of a PGNDesc and returns an instance of it.
// If a pointer to a PatternFrame is included, validate that the asserted type
// is known to the indicated Pattern.  Note therefore that for the purposes of constructing
// an instance of PGNDesc, we don't need to know anything about any Pattern that might use it
func CreatePGNDesc(ptn *PatternFrame, name, node_type string) (PGNDesc, error) {

	// If we do have a pointer to a pattern, check whether the PGNDesc Type is known to the pattern
	// and complain if it is not.   Return a PGNDesc in any case
	var err error = nil

	if ptn != nil && !ptn.PGNDescTypePresent(node_type) {
		err = fmt.Errorf("attempt to create graph node %s in pattern %s with unrecognized type %s",
			name, ptn.Name, node_type)
	}

	gn := new(PGNDesc)
	gn.Name = name
	gn.Type = node_type
	return *gn, err
}

// GraphEdge specifies a child-parent relationship between two ProtocolNodes.
// When the instances connect, each saves a label to be used to find the
// channel through which to communicate with the other.
//
//	The names for the Child and Parent ProtocolNodes are those put into the Name
//
// field of the PGNDesc struct.   The 'Type' of the Child must appear lower
// in the priority order in Pattern's type priority order than the type of the Parent.
// A Pattern may have at most one edge involving the same Child and Parent.
// These conditions are checked for when an attempt is made to create a new GraphEdge

type EdgeDirection int
const (
	Bidirectional EdgeDirection = iota
	Up
	Down
)
type GraphEdge struct {
	Direction EdgeDirection
	ChildName   string
	ParentName  string
	ChildLabel  string // string given to parent to reference child
	ParentLabel string // string given to child to reference parent
}

// Print is used in debugging
func (ge *GraphEdge) Print() {
	fmt.Printf("Graph edge (name %s, label %s), (name %s, label %s) (direction %d)\n",
		ge.ChildName, ge.ChildLabel, ge.ParentName, ge.ParentLabel, ge.Direction)
}

// EdgeEndpoints defines a type used to identify
// and search for edges, based only on the child and parent names, and not the labels
type EdgeEndpoints struct {
	ChildName  string
	ParentName string
}

// Endpoints returns an EdgePoints descriptor for a given Edge
func (ge *GraphEdge) Endpoints() EdgeEndpoints {
	return EdgeEndpoints{ChildName: ge.ChildName, ParentName: ge.ParentName}
}

// CreateGraphEdge makes a GraphEdge, given the child and parent, and labels.
// A type order list is included for a validity check
func CreateGraphEdge(direction EdgeDirection, type_order *[]string, child, parent PGNDesc,
	child_label, parent_label string) (GraphEdge, error) {

	// gather up potentially multiple errors before reporting
	err_slice := []error{}

	// determine the relative positions of parent and child types
	var child_idx int = -1
	var parent_idx int = -1

	// in front to back scan look for declaration of child and parent types, remember their respective indices
	for idx, input_type := range *type_order {
		if input_type == child.Type {
			child_idx = idx
		}
		if input_type == parent.Type {
			parent_idx = idx
		}
	}

	// an index variable that is still -1 was not reset in the scan above
	if child_idx == -1 {
		err_slice = append(err_slice, fmt.Errorf("type %s associated with child %s is missing from type order",
			child.Type, child.Name))
	}

	if parent_idx == -1 {
		err_slice = append(err_slice, fmt.Errorf("type %s associated with parent %s is missing from type order",
			parent.Type, parent.Name))
	}

	// avoid nodes pairing with themselves
	if child.Name == parent.Name {
		err_slice = append(err_slice, fmt.Errorf("node name %s used as both parent and child", child.Name))
	}

	// report any (and all) errors, return an empty GraphEdge
	if len(err_slice) > 0 {
		err := AggregateErrors(err_slice)
		return GraphEdge{}, err
	}

	// check the type ordering relationship, returning an empty GraphEdge and error in case of violation
	if parent_idx < child_idx {
		err := errors.New(fmt.Sprintf("attemped edge (%s, %s) violates type order",
			child.Name, parent.Name))
		return GraphEdge{}, err
	}

	// we're cleared to create this edge
	ge := new(GraphEdge)
	ge.ParentName = parent.Name
	ge.ChildName = child.Name
	ge.ParentLabel = parent_label
	ge.ChildLabel = child_label
	ge.Direction = direction
	return *ge, nil
}

// A parameter group is a json.Marshal created []byte of some structure representing
// a set of parameters.
// type Parameter []byte
type Parameter string

// A ProtocolNode may have multiple groups,
// so Parameters is a map from (unique to node) group name to the encoded group,
// within a common struct type
type ParameterDict map[string]Parameter

// another layer of indexing to build in reference to the type of the struct represented
// by the serialization
type ParameterDictByType map[string]ParameterDict

// we will assign Parameters instances to named nodes within a Pattern
type ParameterGrpsByNode map[string]ParameterDictByType

// PatternFrame is a framework for building a Graph of Protocol Node descriptions (PGNDesc)
// and keeps some data structures used for valiation. After construction we whittle it down
// to a 'Pattern' struct suitable for json exchange
type PatternFrame struct {

	// Each Pattern has a unique name
	Name string

	// A Pattern has an ordered list of Types of the Nodes it organizes.
	// A Type that appears earlier in the list cannot be a parent to a Type
	// that appears later in the list.  At construction the list will be pre-populated
	// with Types that appear in every instance, others can be added later
	PGNDescTypes []string

	// Keep a mapping indicating the type order.  Nodes with Lower valued Types
	// cannot be parents of Nodes with higher valued types
	PGNDescTypeOrder map[string]int

	// The slice of constructed nodes
	PGNDescDict map[string]string

	// a map that given a name indicates whether a PGNDesc instance
	// with that name has been included (yet) in the Pattern
	PGNDescMap map[string]*PGNDesc

	// GraphEdges associate PGNDesc instances via a child-parent relationship
	GraphEdges []GraphEdge

	// given a comparable representation of an edge's endpoints, this
	// map gives a pointer to the PGNDesc
	GraphEdgeMap map[EdgeEndpoints]*GraphEdge
}

// Pattern describes a Graph of Protocol Node descriptions (PGNDesc)
type Pattern struct {

	// Each Pattern has a unique name
	Name string

	// A Pattern has an ordered list of Types of the Nodes it organizes.
	// A Type that appears earlier in the list cannot be a parent to a Type
	// that appears later in the list.  At construction the list will be pre-populated
	// with Types that appear in every instance, others can be added later
	PGNDescTypes []string

	// The dictionary of constructed nodes
	PGNDescDict map[string]string

	// GraphEdges associate PGNDesc instances via a child-parent relationship
	GraphEdges []GraphEdge
}

// Node finds (if one exists) the particular PGNDesc stored in the
// PatternFrame with the offered name
func (ptn *PatternFrame) Node(name string) (PGNDesc, error) {

	pgn_node, present := ptn.PGNDescMap[name]
	if !present {
		err := fmt.Errorf("Node %s not found in Pattern %s",
			name, ptn.Name)
		return PGNDesc{}, err
	}
	return *pgn_node, nil
}

// Edge checks whether a given Pattern has a child-parent relationship
// established between the nodes named in the arguments, and if
// so provides a pointer to it
func (ptn *PatternFrame) Edge(child_name, parent_name string) (GraphEdge, error) {

	// key used to look for the edge existence
	endpts := EdgeEndpoints{ChildName: child_name, ParentName: parent_name}
	edge, present := ptn.GraphEdgeMap[endpts]
	if !present {
		err := fmt.Errorf("Edge (%s,%s) not found in pattern %s",
			child_name, parent_name, ptn.Name)
		// no edge found
		return GraphEdge{}, err
	}
	return *edge, nil
}

// NodePresent determines whether for a given Pattern there is already
// a PGNDesc declared with the given name
func (ptn *PatternFrame) NodePresent(name string) bool {
	_, present := ptn.PGNDescMap[name]
	return present
}

// CreatePatternFrame initializes PatternFrame data structures
func CreatePatternFrame(pattern_name string) PatternFrame {
	ptn := new(PatternFrame)
	ptn.Name = pattern_name

	// incorporate the PGN_Nodes
	ptn.PGNDescMap = make(map[string]*PGNDesc)
	ptn.PGNDescDict = make(map[string]string)

	// incorporate the GraphEdges
	ptn.GraphEdges = make([]GraphEdge, 0)
	ptn.GraphEdgeMap = make(map[EdgeEndpoints]*GraphEdge)

	ptn.PGNDescTypeOrder = make(map[string]int)

	// default ordering of types
	ptn.CreateOrderedTypes(GlobalpgnDescTypes)
	return *ptn
}

// CreateOrderedTypes transforms an input list of strings
// into an expression of ordered types.  Can be used to reset
// the default after construction.  Return error if duplication expressed
func (ptn *PatternFrame) CreateOrderedTypes(types []string) error {

	// initialize impacted data structures
	ptn.PGNDescTypes = make([]string, 0)
	ptn.PGNDescTypeOrder = make(map[string]int)

	// for each given type, in order, add to the PatternFrames
	// list of types and record its relative position
	for idx, pgn_type := range types {

		// if we've already registered this type flag an error
		_, present := ptn.PGNDescTypeOrder[pgn_type]
		if present {
			// don't leave partial results behind
			ptn.PGNDescTypes = make([]string, 0)
			ptn.PGNDescTypeOrder = make(map[string]int)

			// complain
			err := errors.New(fmt.Sprintf("duplicate node type"))
			return err
		}
		// do the needful
		ptn.PGNDescTypes = append(ptn.PGNDescTypes, pgn_type)
		ptn.PGNDescTypeOrder[pgn_type] = idx
	}
	return nil
}

// CreatePattern extracts the information within PatternFrame that is
// part of the initialization (as opposed to build) interface and
// returns an instance of the Pattern struct that results
func (ptnf *PatternFrame) CreatePattern() Pattern {
	ptn := new(Pattern)
	ptn.Name = ptnf.Name
	ptn.PGNDescDict = ptnf.PGNDescDict
	ptn.GraphEdges = ptnf.GraphEdges
	ptn.PGNDescTypes = ptnf.PGNDescTypes

	return *ptn
}

func (ptn *PatternFrame) AddPGNDesc(node_name, node_type string) (PGNDesc, error) {
	pgn_desc, err := CreatePGNDesc(ptn, node_name, node_type)
	ptn.PGNDescMap[node_name] = &pgn_desc
	ptn.PGNDescDict[pgn_desc.Name] = pgn_desc.Type
	return pgn_desc, err
}

// AddEdge takes pointers to PGN_Nodes (potentially) assigned child and parent
// roles and labels to be used for them, and tries to make a GraphEdge from them
// An error is returned if the types of child and parent prohibit that association,
// or if this Edge was already created once
func (ptn *PatternFrame) AddEdge(child, parent PGNDesc,
		child_label, parent_label string) (GraphEdge, error) {
	return ptn.AddDirectedEdge(Bidirectional, child, parent, child_label, parent_label)
} 

// AddEdge takes pointers to PGN_Nodes (potentially) assigned child and parent
// roles and labels to be used for them, and tries to make a GraphEdge from them
// An error is returned if the types of child and parent prohibit that association,
// or if this Edge was already created once
func (ptn *PatternFrame) AddDirectedEdge(direction EdgeDirection, child, parent PGNDesc,
	child_label, parent_label string) (GraphEdge, error) {

	// CreateGraphEdge will do the priority order test and complain on failure.
	// Otherwise variable 'edge' has the result
	edge, err := CreateGraphEdge(direction, &ptn.PGNDescTypes, child, parent, child_label, parent_label)
	edge.Direction = direction

	if err != nil {
		return GraphEdge{}, err
	}

	// But it may still be the case that an edge between this child and parent
	// already exists.  Check

	// create the index we use to look for duplication using the GraphEdgeMap
	ge_endpts := EdgeEndpoints{ChildName: child.Name, ParentName: parent.Name}

	_, present := ptn.GraphEdgeMap[ge_endpts]

	// this edge has already been declared
	if present {
		err := fmt.Errorf("attempt to reintroduce edge (%s,%s) to traffic pattern %s",
			child.Name, parent.Name, ptn.Name)
		return GraphEdge{}, err
	}

	// Checks passed, OK to include and put into the GraphEdgeMap
	ptn.GraphEdges = append(ptn.GraphEdges, edge)
	ptn.GraphEdgeMap[ge_endpts] = &edge
	return edge, nil
}


// EndptCfgFrame describes the components of a EndptCfg, plus auxilary data structures
// that are discared before Serialization
type EndptCfgFrame struct {
	EndpointName string
	PatternName  string
	Pattern_ptr  *PatternFrame
	ExcRate      float64 // in 'work units per second', work units are user implied by parameter choice
	MemSize      float64 // execution rate can be impacted when memory is over-committed
	PGNByNode    ParameterGrpsByNode
}

// EndptCfg describes the serializable expression of how an endpoint is to be configured
type EndptCfg struct {

	// name of the Endpoint from the Topology point of view
	EndpointName string

	// Name of the Pattern used at that Endpoint
	PatternName string

	ExcRate float64 // in 'work units per second', work units are user implied by parameter choice
	MemSize float64 // execution rate can be impacted when memory is over-committed

	// Map of Pattern-centric PGNDesc node names to lists of parameter-value
	// pairs to pass to a constructor
	PGNByNode ParameterGrpsByNode
}

// CreateEndptCfg transforms an EndptCfgFrame into serializable form
func (epcf *EndptCfgFrame) CreateEndptCfg() EndptCfg {
	epc := new(EndptCfg)
	epc.EndpointName = epcf.EndpointName
	epc.PatternName = epcf.PatternName
	epc.ExcRate = epcf.ExcRate
	epc.MemSize = epcf.MemSize

	epc.PGNByNode = epcf.PGNByNode
	return *epc
}

// CreateEndptCfgFrame constructs the data structures for an
// instance of the EndptCfgFrame struct. Note that a pointer to the Pattern named
// in the instance is required, implying that the PatternFrame has already been constructed
func CreateEndptCfgFrame(ptn *PatternFrame, endpt_name string) EndptCfgFrame {
	epcf := new(EndptCfgFrame)
	epcf.EndpointName = endpt_name
	epcf.Pattern_ptr = ptn
	epcf.PatternName = ptn.Name
	epcf.PGNByNode = make(ParameterGrpsByNode)
	return *epcf
}

// Given a node name, group name, and parameter group encoding,
// add the parameter group to the node's configuration map
func (epc *EndptCfgFrame) AddParameterGrp(node_name string, grp_name string, pdg_struct any, warn_overwrite bool) error {
	// return an error if the referenced node name is not recognized
	if !epc.Pattern_ptr.NodePresent(node_name) {
		err := fmt.Errorf("attempt to give parameter to unrecognized node %s grp %s in pattern %s",
			node_name, grp_name, epc.PatternName)
		fmt.Printf("attempt to give parameter to unrecognized node %s grp %s in pattern %s",
			node_name, grp_name, epc.PatternName)
		return err
	}

	// ensure that the submitted parameter group structure serializes
	pdg, err := json.MarshalIndent(pdg_struct, "", "\t")
	if err != nil {
		fmt.Println("Parameter Group does not serialize")
		return err
	}

	pdg_name := "lost"
	// get the type of the structure being serialized
	if t := reflect.TypeOf(pdg_struct); t.Kind() == reflect.Ptr {
		pdg_name = fmt.Sprintf("%s", t.Elem())
	}

	// if needed, initialize the node's map of group structure type to dictionary of groups with that structure type
	_, present_by_node := epc.PGNByNode[node_name]
	if !present_by_node {
		epc.PGNByNode[node_name] = make(ParameterDictByType)
	}

	// if needed, initialize the map of structure type names
	_, present_by_struct := epc.PGNByNode[node_name][pdg_name]
	if !present_by_struct {
		epc.PGNByNode[node_name][pdg_name] = make(map[string]Parameter)
	}

	// default group name is "cfg"
	if len(grp_name) == 0 {
		grp_name = "cfg"
	}

	// if we have enabled warning on an an overwrite (overwrite is useful for copying)
	// return an error if this node already has a parameter group with the given name
	if warn_overwrite {
		_, presentBy := epc.PGNByNode[node_name][pdg_name][grp_name]
		if presentBy {
			err := fmt.Errorf("attempt to overwrite group %s on node %s", grp_name, node_name)
			fmt.Printf("attempt to overwrite group %s on node %s", grp_name, node_name)
			return err
		}
	}

	// save the serialization
	epc.PGNByNode[node_name][pdg_name][grp_name] = Parameter(string(pdg))
	return nil
}

// CreateSysEndptCfg creates a SysEndptCfg from a lists of EndptCfgFrames and PatternFrames
// This basically involves transforming the list elements into serializable representations
func CreateSysEndptCfg(epcfs []EndptCfgFrame, ptns []PatternFrame) SysEndptCfg {
	ef := new(SysEndptCfg)

	ef.PatternMap = make(map[string]Pattern)
	ef.EndptCfgs = make(map[string]EndptCfg)

	// create the Patterns from PatternFrames and store them
	for _, ptnf := range ptns {
		ptn := ptnf.CreatePattern()
		ef.PatternMap[ptn.Name] = ptn
	}

	// create the map of EndptCfgs (indexed by node name)
	for _, epcf := range epcfs {
		ef.EndptCfgs[epcf.EndpointName] = epcf.CreateEndptCfg()
	}

	return *ef
}

// Serialize transforms the configuration structure contained in a given SysEndptCfg struct
// into a slice of bytes created using the json Marshal functionality
func (tf *SysEndptCfg) Serialize() ([]byte, error) {
	bytes, merr := json.MarshalIndent(*tf, "", "\t")
	if merr != nil {
		return nil, merr
	} else {
		return bytes, nil
	}
}

// AggregateErrors lets us gather a slice of individual errors
// into one, where the individual error messages appear
// in comman-separated form
func AggregateErrors(error_slice []error) error {
	err_msg := make([]string, 0)
	for _, err := range error_slice {
		if err != nil {
			err_msg = append(err_msg, fmt.Sprintf("%s", err))
		}
	}
	if len(err_msg) == 0 {
		return nil
	}
	new_err := errors.New(strings.Join(err_msg, "\n"))
	return new_err
}

type RtrDescSlice []RouterDesc
type EndpointDescSlice []EndpointDesc
type NetworkDescSlice []NetworkDesc

// type OfflineSrc_desc_slice []OfflineSrcDesc
type IPMapDescSlice []IPMapDesc
type IPRemapDescSlice []IPRemapDesc

// Topodict contains all of the networks, routers, and
// endpoints, as they are listed in the json file.
type Topodict struct {
	Networks  NetworkDescSlice  `json:"Networks"`
	Routers   RtrDescSlice      `json:"Routers"`
	Endpoints EndpointDescSlice `json:"Endpoints"`
}

type IPMapDict struct {
	Map IPMapDescSlice `json:"Map"`
}

type IPRemapDict struct {
	Remap IPRemapDescSlice `json:"Remap"`
}

type SysEndptCfg struct {
	PatternMap map[string]Pattern
	EndptCfgs  map[string]EndptCfg
}

type IPMapDesc struct {
	Name string
	IP   string
}

type IPRemapDesc struct {
	Original string
	Remapped string
}

// types for functions called to build from code the specifics of a
// system. TopoBuild creates the network and identifies endpoints
type TopoDescBuild func(any) (*Topodict, error)

// EndptBuild creates protocol node graphs for the endpoints described
// in the topology dictionary
type EndptDescBuild func(any, *Topodict) (*SysEndptCfg, error)

// IPMapBuild creates an assignment of IP addresses to networks and
// Endpoint interfaces described in the topology and endpoint dictionaries
type IPMapBuild func(any, *Topodict, *SysEndptCfg) (*IPMapDict, error)

// IPMapReBuild creates a re-assignment of selected IP addresses to networks and
// Endpoint interfaces described in the topology and endpoint dictionaries
type IPRemapBuild func(any, *Topodict, *SysEndptCfg) (*IPRemapDict, error)

// before calling the function to create the system
// the user can set the identifies of the functions
// to be called, put in the various "BuildFunc" variables below
var TopoDescBuildFunc TopoDescBuild
var EndptDescBuildFunc EndptDescBuild
var IPMapBuildFunc IPMapBuild
var IPRemapBuildFunc IPRemapBuild

// Each build function can be passed a map for its inputs,
// the idea being that inputs are given as attribute-value maps
// and these can be specified by the user before the function call
var TopoDescArgs any
var EndptDescArgs any
var IPMapArgs any
var IPRemapArgs any

func SetTopoDescBuildFunc(topo_desc_build_func TopoDescBuild) {
	TopoDescBuildFunc = topo_desc_build_func
}

func SetEndptDescBuildFunc(endpt_desc_build_func EndptDescBuild) {
	EndptDescBuildFunc = endpt_desc_build_func
}

func SetIPMapBuildFunc(ip_map_build_func IPMapBuild) {
	IPMapBuildFunc = ip_map_build_func
}

func SetIPRemapBuildFunc(ip_remap_build_func IPRemapBuild) {
	IPRemapBuildFunc = ip_remap_build_func
}

func SetTopoDescArgs(args any) {
	TopoDescArgs = args
}

func SetEndptDescArgs(args string) {
	EndptDescArgs = args
}

func SetIPMapArgs(args any) {
	IPMapArgs = args
}

func SetIPRemapArgs(args any) {
	IPRemapArgs = args
}

// ProtocolNode initialization parameters
//
// Each ProtocolNode is initialized with a node-type specific
// set of initialization parameters.  Parameters are gathered into
// groups, so a ProtocolNode's initialization includes access
// to an instance of each group created for that node.
//   Each group type has a struct type defined.  To create
// a group the user fills in an in instance of that struct
// and calls AddParameter to include it.  When
// the serialized instantiation is deserialized, a flesh-out
// instance of that group is returned
//
// Common parameter groups are defined below.  The name includes
// the type of ProtocolNode to which the group is attached

type NicCfg struct {
	DeviceName string  // Name of the device (endpoint or router) to which this NIC is attached
	LocalName  string  // Differentiates between NICS on the same device
	Bandwidth  float64 // bandwidth of interface governing flow in and out.
	// If non-zero overwrites the default placed by constructor
	BufferSize float64 // can be used to have message size and flows govern behavior
	Faces      string  // name of network to which this NIC is connected
	IP         string
}

type NetworkCfg struct {
	IP_map  map[string]string // NIC@Endpoint to IP address
	IPremap map[string]string // given an IP address in the PGN_Msg header, remap to another
}

type EndpointCfg struct {
	ExcRate float64 // in 'work units per second', work units are user implied by parameter choice
	MemSize float64 // execution rate can be impacted when memory is over-committed
}

// Endpt_perf_params holds the parameters of an executing endpoint, used for computing resource consumption
type EndptExcCfg struct {
	ExcRate float64 // in 'work units per second', work units are user implied by parameter choice
	MemSize float64 // execution rate can be impacted when memory is over-committed
}

type TransportCfg struct {
	Dummy bool
}

// through is very general and needs to be defined for user specific developed nodes.
// The default model is to include a delay and execution rate just to have a connection
// impact the resource consumption and its impact on other activities
type ThruPerf struct {
	ExcRate float64 // in work-units / second
	MemReq  float64 // amount of memory used by this node
}

type MeasureConvertCfg struct {
	units         string
	unitsToSecMax float64
	secsToUnits   float64
}

// Offline reads pcap packets from a file, waits a while, and then does it again (and on...)
type OfflineCfg struct {
	Filename             string            // name of file containing pcap trace
	Active          bool              // trace source is actively generating packets
	SrcIP                string            // source IP address of pcap entries to include in the trace
	RemapIP              map[string]string // remapping of IP addresses coming from trace source to topological IP address space
	OffMeasureDist       string            // distribution of time between packets pulled from trace source
	OffMeasureDistParams []float64         // parmeters for inter-arrival response time
	OffMeasureType       string            // off period based on "Workload" or "Time"
	OffWorkloadRateMax   float64           // Maximum rate of executing workload during an off-period
	Repeat               bool              // store and recycle traces to have something to kick out when exhausted
}

type OnlineCfg struct {
	DeviceName           string            // Name of Linux network device
	SrcIP                string            // source IP address of pcap entries to include in the trace
	Active               bool              // we doing this one?
	RemapIP              map[string]string // remapping of IP addresses coming from trace source to topological IP address space
	OffMeasureDist       string
	OffMeasureDistParams []float64
	OffMeasureType       string  // off period based on "Workload" or "Time"
	OffWorkloadRateMax   float64 // Maximum rate of executing workload during an off-period
}

// rateflow_params holds the parameters of a rate source description
type RateflowSetCfg struct {
	SrcName     string // name of Endpoint to which this node is attached
	MemReq      float64
	Active bool   //
	DstDist     string // Distribution to use to sample a destination for a flow
	// "All" is a possibility as is "Multinomial"
	DstNames             []string  // List of destination names over which to sample
	DstDistParams        []float64 // Parameters of the destination sampling distribution
	OnMeasureDist        string    // Distribution of the workload (alt comm load) for an on period
	OnMeasureDistParams  []float64 // Parameters of the on-period measure sampling distribution
	OnMeasureType        string    // On-measure type, either "Workload" or "Communication"
	CommWorkloadRatio    float64   // Communication generated per unit workload during an on period
	WorkloadCommRatio    float64   // workload per unit comm during an on period
	OnWorkloadRateMax    float64   // Fastest rate workload can be executed during an on period
	OffMeasureDist       string    // Distribution of measure used to govern off-period
	OffMeasureDistParams []float64 // Parameters for the off-measure time distribution
	OffMeasureType       string    // Type of off-period measure, either "Workload" or "Time"
	OffWorkloadRateMax   float64   // Maximum rate of executing workload during an off-period
}

type FlowReduction struct {
	DstName   string
	Reduction float64
}

type DelayNodeCfg struct {
	UpDelayWrkld float64
	DownDelayWrkld float64
}


type CongestionCfg struct {
	SrcName        string // name of Endpoint to which this node is attached
	Active    bool   //
	RisingState    bool
	CgnLevelPeriod float64 // in seconds
	MinCgnRate     float64 // in MB/sec
	MaxCgnRate     float64 // in MB/sec
	CgnChangeSteps int
	InitialCgnRate float64 // in MB/sec
	Tgts           []FlowReduction
}

// the Sim type is like the rate flow type except that it generates a packet, not
// a flow.  It has randomized destination and randomized inter-arrival time
type SimCfg struct {
	SrcName              string    // the name of the Endpoint from which this traffic is sent
	CommType             string    // Pt2P2, Multicast, or Broadcast
	DstDist              string    // type of distribution to use when selecting a destination
	DstNames             []string  // names of endpoints to receive copies of a send
	SelectionDistParams  []float64 // parameters for the selection distribution
	OffMeasureDist       string    // distribution of inter-arrival measure
	OffMeasureDistParams []float64 // parameters for distribution
	OffMeasureType       string    // "Workload" or "Time", depending on what we measure to trigger next send
	OffWorkloadRateMax   float64   // draw rate on CPU resources when the inter-arrival type is "Workload"
}

type ServiceDesc struct {
	FlowType string
	Port     int
}

// the sim type is like the rate flow type except that it generates a packet, not
// a flow.  It has randomized destination and randomized inter-arrival time
type ServerCfg struct {
	EndptName               string        // the name of the Endpoint where this server resides
	Services                []ServiceDesc // description of services offered
	SrvPort                 int           // destination port advertising this service
	ServiceLengthDist       string        // distribution of measure governing the response to a request
	ServiceLengthDistParams []float64     // parameters for distribution
	ServiceLengthType       string        // "Workload" or "Time", depending on what we use to govern length of response
	ServiceLengthRateMax    float64       // maximum draw rate on CPU resources when the inter-arrival type is "Workload"
}

func RecoverParameter(pgrp Parameter, istruct any) error {
	return json.Unmarshal([]byte(string(pgrp)), istruct)

}
