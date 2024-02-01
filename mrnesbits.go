package mrnesbits

// sys.go has code that builds the system data structures

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/mrnes"
	"path"
)

// interface to network simulator
type NetSimPortal interface {
	CreateNetworkPortal(bool) NetSimPortal
	HostCPU(string) string
	EnterNetwork(*evtm.EventManager, string, string, int, float64, any, any, evtm.EventHandlerFunction) any
}

var netportal *mrnes.NetworkPortal

// declare global variables that are loaded from
// analysis of input files
var cmpPtnMapDict *CompPatternMapDict
var funcExecTimeTbl map[string]map[string]map[int]float64

// A cmpPtnGraph is the run-time description of a CompPattern
type cmpPtnGraph struct {

	// every instance a function has a string 'label', used here to index to
	// data structure that describes it and the connections it has with other funcs
	nodes map[string]*cmpPtnGraphNode
}

// A cmpPtnGraph edge declares the possibility that a function with label srcLabel
// might send a message of type msgType to the function (in the same CPG) with label dstLabel
type cmpPtnGraphEdge struct {
	srcLabel  string
	msgType   string
	dstLabel  string
	edgeLabel string
}

// A cmpPtnGraphNode names a function with its label, and describes
// the edges for which it is a destination (inEdges) and edges
// for which it is a source (outEdges)
type cmpPtnGraphNode struct {
	label    string
	inEdges  []*cmpPtnGraphEdge
	outEdges []*cmpPtnGraphEdge
}

// addEdge takes the description of a CPG edge and attempts
// to add that edge to the CPG node corresponding to the edge's source
func (cpg *cmpPtnGraph) addEdge(srcLabel, msgType, dstLabel, edgeLabel string) error {

	// ensure that cpg map of nodes includes the edge's source lable
	pgn, present := cpg.nodes[srcLabel]
	if !present {
		pgn = createCmpPtnGraphNode(srcLabel)
		cpg.nodes[srcLabel] = pgn
	}

	// add the edge to node with label srcLabel, return any err generated
	err := pgn.addEdge(srcLabel, msgType, dstLabel, edgeLabel)
	return err
}

// addEdge takes the description of a CPG edge, builds a representation of it,
// and includes the CPG node's list of input edges. An error is returned if
// an attempt is made to add an edge that is already present
func (pgn *cmpPtnGraphNode) addEdge(srcLabel, msgType, dstLabel, edgeLabel string) error {
	edge := &cmpPtnGraphEdge{srcLabel: srcLabel, msgType: msgType, dstLabel: dstLabel, edgeLabel: edgeLabel}

	// source and destination labels are special case, indicating initiation.
	// plunk it into the list of inEdges and be done with it.
	if srcLabel == dstLabel {
		for _, inedge := range pgn.inEdges {
			if *edge == *inedge {
				return fmt.Errorf("attempt to add in-edge already present\n")
			}
		}
		pgn.inEdges = append(pgn.inEdges, edge)
		return nil
	}

	// otherwise determine whether this is an 'in' edge or an 'out' edge
	if pgn.label == srcLabel {
		// see whether edge is in this list already
		for _, outedge := range pgn.outEdges {
			if *outedge == *edge {
				return fmt.Errorf("attempt to add out-edge already present\n")
			}
		}
		pgn.outEdges = append(pgn.outEdges, edge)
	} else if pgn.label == dstLabel {
		for _, inedge := range pgn.inEdges {
			if *inedge == *edge {
				return fmt.Errorf("attempt to add in-edge already present\n")
			}
		}
		pgn.inEdges = append(pgn.inEdges, edge)
	}
	return nil
}

// createCmpPtnGraph builds a graph representation for an input CompPattern Type
func createCmpPtnGraph(cp *CompPattern) (*cmpPtnGraph, error) {
	var errList []error
	cpg := new(cmpPtnGraph)
	cpg.nodes = make(map[string]*cmpPtnGraphNode)

	// note that nodes are inferred from edges, not pulled out explicitly first
	// Edge is (SrcLabel, DstLabel, MsgType) where the labels are for funcs
	for _, edge := range cp.Edges {
		err := cpg.addEdge(edge.SrcLabel, edge.MsgType, edge.DstLabel, edge.EdgeLabel)
		errList = append(errList, err)
	}
	return cpg, ReportErrs(errList)
}

// createCmpPtnGraphNode is a constructor, saving the label and initializing the slices
func createCmpPtnGraphNode(label string) *cmpPtnGraphNode {
	pgn := new(cmpPtnGraphNode)
	pgn.label = label
	pgn.inEdges = make([]*cmpPtnGraphEdge, 0)
	pgn.outEdges = make([]*cmpPtnGraphEdge, 0)
	return pgn
}

// buildCmpPtns goes through every CompPattern in the input CompPatternDict,
// and creates a run-time cmpPtnInst representation for it.
func buildCmpPtns(cpd *CompPatternDict, cpid *CPInitListDict) error {

	errList := []error{}
	// CompPatterns are arranged in a map that is index by the CompPattern name
	for cpName, cp := range cpd.Patterns {

		// use the CompPattern name to index to the correct member
		// of the map of comp pattern initialization structs
		cpi, err := createCmpPtnInst(cpName, cp, cpid.InitList[cpName])
		errList = append(errList, err)

		// save the instance we we can look it up given the comp pattern name
		cmpPtnInstByName[cpName] = cpi
	}
	return ReportErrs(errList)
}

// buildFuncExecTimeTbl creates a very nested map that stores information
// about comp pattern function execution times.  The keys in order of application are
//
//	map[function type] -> map[cpu type] -> map[message length] -> execution time
func buildFuncExecTimeTbl(fel *FuncExecList) map[string]map[string]map[int]float64 {
	et := make(map[string]map[string]map[int]float64)

	// loop over the primary key, function type
	for funcType, mapList := range fel.Times {

		// make sure that we've initialized a map for the map[funcType] value
		_, present := et[funcType]
		if !present {
			et[funcType] = make(map[string]map[int]float64)
		}

		// given the function type, the associated mapList is a list of
		// structs that associate attribute information (cpu type, packet len) with the execution time
		for _, funcExecDesc := range mapList {
			cpuType := funcExecDesc.ProcessorType
			pcktLen := funcExecDesc.PcktLen
			execTime := funcExecDesc.ExecTime

			// use the cpuType and pckLen attributes to flesh out the execution time table
			_, present := et[funcType][cpuType]
			if !present {
				et[funcType][cpuType] = make(map[int]float64)
			}

			et[funcType][cpuType][pcktLen] = execTime
		}
	}
	return et
}

// BuildExperiment is called from the module that creates and runs
// a simulation. Its inputs identify the names of input files, which it
// uses to assemble and initialize the model (and experiment) data structures.
// It returns a pointer to an EventManager data structure used to coordinate the
// execution of events in the simulation.
func BuildExperimentCP(syn map[string]string, useYAML bool) (*evtm.EventManager, error) {
	// syn is a map that binds pre-defined keys referring to input file types with file names
	// The keys are
	//	"cpInput"		- file describing comp patterns and functions
	//	"cpInitInput"	- file describing initializations for comp patterns and functions
	//	"funcExecInput"		- file describing function and device operation execution timing
	//	"mapInput"		- file describing the mapping of functions to hosts

	// "qksim" if present means to tell the network portal to use quick simulation

	// call GetExperimentCPDicts to do the heavy lifting of extracting data structures
	// (typically maps) designed for serialization/deserialization,  and assign those maps to variables
	// we'll use to re-represent this information in structures optimized for run-time use
	cpd, cpid, fel, cpmd := GetExperimentCPDicts(syn)

	// get a pointer to a mrns NetworkPortal
	_, use := syn["qksim"]
	netportal = mrnes.CreateNetworkPortal(use)

	// panic if any one of these dictionaries could not be built
	if (cpd == nil) || (cpid == nil) || (fel == nil) || (cpmd == nil) {
		panic("empty dictionary")
	}

	// remember the mapping of functions to host
	cmpPtnMapDict = cpmd

	// build the tables used to look up the execution time of comp pattern functions, and device operations
	funcExecTimeTbl = buildFuncExecTimeTbl(fel)

	// create the run-time representation of comp patterns, and initialize them
	err := buildCmpPtns(cpd, cpid)

	// build the response function table
	buildRespTbl()

	// schedule the initiating events on self-initiating funcs
	evtMgr := evtm.New()
	schedInitEvts(evtMgr)
	return evtMgr, err
}

// utility function for generating unique integer ids on demand
var numIds int = 0

func nxtId() int {
	numIds += 1
	return numIds
}

// GetExperimentDicts accepts a map that holds the names of the input files used to define an experiment,
// creates internal representations of the information they hold, and returns those structs.
func GetExperimentCPDicts(syn map[string]string) (*CompPatternDict, *CPInitListDict, *FuncExecList, *CompPatternMapDict) {
	var cpd *CompPatternDict
	var cpid *CPInitListDict
	var fel *FuncExecList
	var cpmd *CompPatternMapDict

	var empty []byte = make([]byte, 0)

	var errs []error
	var err error

	var useYAML bool
	ext := path.Ext(syn["cpInput"])
	useYAML = (ext == ".yaml") || (ext == ".yml")

	cpd, err = ReadCompPatternDict(syn["cpInput"], useYAML, empty)
	errs = append(errs, err)

	ext = path.Ext(syn["cpInitInput"])
	useYAML = (ext == ".yaml") || (ext == ".yml")

	cpid, err = ReadCPInitListDict(syn["cpInitInput"], useYAML, empty)
	errs = append(errs, err)

	ext = path.Ext(syn["funcExecInput"])
	useYAML = (ext == ".yaml") || (ext == ".yml")

	fel, err = ReadFuncExecList(syn["funcExecInput"], useYAML, empty)
	errs = append(errs, err)

	ext = path.Ext(syn["mapInput"])
	useYAML = (ext == ".yaml") || (ext == ".yml")

	cpmd, err = ReadCompPatternMapDict(syn["mapInput"], useYAML, empty)
	errs = append(errs, err)

	err = ReportErrs(errs)
	if err != nil {
		panic(err)
	}

	return cpd, cpid, fel, cpmd
}
