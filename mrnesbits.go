package mrnesbits

// sys.go has code that builds the system data structures

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
	"github.com/iti/mrnes"
	"path"
	"strings"
)

// interface to network simulator
type NetSimPortal interface {
	HostCPU(string) string
	EnterNetwork(*evtm.EventManager, string, string, int, int, float64, any,
		any, evtm.EventHandlerFunction, any, evtm.EventHandlerFunction) any
}

// The TraceManager interface helps integrate use of the mrnes functionality for managing
// traces in this (different) model
type TraceManager interface {

	// at creation a flag is set indicating whether the trace manager will be active
	Active() bool

	// add a trace event to the manager
	AddTrace(vrtime.Time, int, int, int, string, bool, float64)

	// include an id -> (name, type) pair in the trace manager dictionary
	AddName(int, string, string)

	// save the trace to file (if active) and return flag indicating whether file creatation actually happened
	WriteToFile(string) bool
}

// pointers that are global to mrnesbits to mrnes implemenations of the NetworkPortal and TraceManager interfaces
var netportal *mrnes.NetworkPortal
var traceMgr TraceManager

// declare global variables that are loaded from
// analysis of input files
var CmpPtnMapDict *CompPatternMapDict
var funcExecTimeTbl map[string]map[string]map[int]float64

// A CmpPtnGraph is the run-time description of a CompPattern
type CmpPtnGraph struct {

	// every instance a function has a string 'label', used here to index to
	// data structure that describes it and the connections it has with other funcs
	Nodes map[string]*CmpPtnGraphNode
}

// A CmpPtnGraph edge declares the possibility that a function with label srcLabel
// might send a message of type msgType to the function (in the same CPG) with label dstLabel
type CmpPtnGraphEdge struct {
	SrcLabel  string
	MsgType   string
	DstLabel  string
	EdgeLabel string
}

func CreateCmpPtnGraphEdge(srcLabel, msgType, dstLabel, edgeLabel string) CmpPtnGraphEdge {
	edge := CmpPtnGraphEdge{SrcLabel: srcLabel, MsgType: msgType, DstLabel: dstLabel, EdgeLabel: edgeLabel}
	return edge
}

// A CmpPtnGraphNode names a function with its label, and describes
// the edges for which it is a destination (inEdges) and edges
// for which it is a source (outEdges)
type CmpPtnGraphNode struct {
	Label    string
	InEdges  []*CmpPtnGraphEdge
	OutEdges []*CmpPtnGraphEdge
}

// addEdge takes the description of a CPG edge and attempts
// to add that edge to the CPG node corresponding to the edge's source and destination
func (cpg *CmpPtnGraph) addEdge(srcLabel, msgType, dstLabel, edgeLabel string) error {
	edge := &CmpPtnGraphEdge{SrcLabel: srcLabel, MsgType: msgType, DstLabel: dstLabel, EdgeLabel: edgeLabel}

	srcNode, present1 := cpg.Nodes[srcLabel]
	if !present1 {
		return fmt.Errorf("srcLabel offered to addEdge not recognized")
	}

	dstNode, present2 := cpg.Nodes[dstLabel]
	if !present2 {
		return fmt.Errorf("dstLabel offered to addEdge not recognized")
	}

	// add to srcNode outEdges, if needed, and if not initiation
	var duplicated bool = false
	for _, outEdge := range srcNode.OutEdges {
		if *outEdge == *edge {
			duplicated = true
			break
		}
	}

	if !duplicated && srcLabel != dstLabel {
		srcNode.OutEdges = append(srcNode.OutEdges, edge)
	}

	// add to dstNode inEdges, if needed
	duplicated = false
	for _, inEdge := range dstNode.InEdges {
		if *inEdge == *edge {
			duplicated = true
			break
		}
	}

	if !duplicated {
		dstNode.InEdges = append(dstNode.InEdges, edge)
	}

	return nil
}

// createCmpPtnGraph builds a graph representation for an input CompPattern Type
func createCmpPtnGraph(cp *CompPattern) (*CmpPtnGraph, error) {
	var errList []error
	cpg := new(CmpPtnGraph)
	cpg.Nodes = make(map[string]*CmpPtnGraphNode)

	// create a node for every func in the comp pattern
	for _, funcName := range cp.Funcs {
		label := funcName.Label
		cpg.Nodes[label] = createCmpPtnGraphNode(label)
	}

	// note that nodes are inferred from edges, not pulled out explicitly first
	// Edge is (SrcLabel, DstLabel, MsgType) where the labels are for funcs
	for _, edge := range cp.Edges {
		err := cpg.addEdge(edge.SrcLabel, edge.MsgType, edge.DstLabel, edge.EdgeLabel)
		errList = append(errList, err)
	}
	return cpg, ReportErrs(errList)
}

// createCmpPtnGraphNode is a constructor, saving the label and initializing the slices
func createCmpPtnGraphNode(label string) *CmpPtnGraphNode {
	pgn := new(CmpPtnGraphNode)
	pgn.Label = label
	pgn.InEdges = make([]*CmpPtnGraphEdge, 0)
	pgn.OutEdges = make([]*CmpPtnGraphEdge, 0)
	return pgn
}

// buildCmpPtns goes through every CompPattern in the input CompPatternDict,
// and creates a run-time CmpPtnInst representation for it.
func buildCmpPtns(cpd *CompPatternDict, cpid *CPInitListDict) error {

	errList := []error{}
	// CompPatterns are arranged in a map that is indexed by the CompPattern name
	for cpName, cp := range cpd.Patterns {

		// use the CompPattern name to index to the correct member
		// of the map of comp pattern initialization structs
		var cpi *CmpPtnInst
		var err error
		cpi, err = createCmpPtnInst(cpName, cp, cpid.InitList[cpName])

		errList = append(errList, err)

		// save the instance we can look it up given the comp pattern name
		CmpPtnInstByName[cpName] = cpi
	}
	return ReportErrs(errList)
}

// buildFuncExecTimeTbl creates a very nested map that stores information
// about comp pattern function execution times.  The keys in order of application are
//
//	map[operation] -> map[hardware] -> map[message length] -> execution time
func buildFuncExecTimeTbl(fel *FuncExecList) map[string]map[string]map[int]float64 {
	et := make(map[string]map[string]map[int]float64)

	// loop over the primary key, function type
	for operation, mapList := range fel.Times {

		// make sure that we've initialized a map for the map[operation] value
		_, present := et[operation]
		if !present {
			et[operation] = make(map[string]map[int]float64)
		}

		// given the function class, the associated mapList is a list of
		// structs that associate attribute information (cpu type, packet len) with the execution time
		for _, funcExecDesc := range mapList {
			hardware := funcExecDesc.Hardware
			pcktLen := funcExecDesc.PcktLen
			execTime := funcExecDesc.ExecTime

			// use the cpuType and pckLen attributes to flesh out the execution time table
			_, present := et[operation][hardware]
			if !present {
				et[operation][hardware] = make(map[int]float64)
			}

			et[operation][hardware][pcktLen] = execTime
		}
	}
	return et
}

// BuildExperiment is called from the module that creates and runs
// a simulation. Its inputs identify the names of input files, which it
// uses to assemble and initialize the model (and experiment) data structures.
// It returns a pointer to an EventManager data structure used to coordinate the
// execution of events in the simulation.
func BuildExperimentCP(syn map[string]string, useYAML bool, idCounter int, tm TraceManager) (*evtm.EventManager, error) {
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

	netportal = mrnes.CreateNetworkPortal()
	_, use := syn["qksim"]
	netportal.SetQkNetSim(use)

	// panic if any one of these dictionaries could not be built
	if (cpd == nil) || (cpid == nil) || (fel == nil) || (cpmd == nil) {
		panic("empty dictionary")
	}

	NumIds = idCounter
	traceMgr = tm

	// remember the mapping of functions to host
	CmpPtnMapDict = cpmd

	// build the tables used to look up the execution time of comp pattern functions, and device operations
	funcExecTimeTbl = buildFuncExecTimeTbl(fel)

	// create the run-time representation of comp patterns, and initialize them
	err := buildCmpPtns(cpd, cpid)

	// schedule the initiating events on self-initiating funcs
	evtMgr := evtm.New()
	schedInitEvts(evtMgr)
	return evtMgr, err
}

// utility function for generating unique integer ids on demand
var NumIds int = 0

func nxtId() int {
	NumIds += 1
	return NumIds
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

	// we allow some variation in input names, so apply fixup if needed
	checkFields := []string{"cpInput", "cpInitInput", "funcExecInput", "mapInput"}
	for _, filename := range checkFields {
		trimmed := strings.Replace(filename, "Input", "", -1)
		_, present := syn[trimmed]
		if present {
			syn[filename] = syn[trimmed]
		}
	}

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

func ReportStatistics() {
	for _, cpi := range CmpPtnInstByName {
		cpi.ExecReport()
	}
}
