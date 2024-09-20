package pces

// pces.go has code that builds mrnes system data structures

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
	"github.com/iti/mrnes"
	"path"
	"sort"
	"strings"
)

// NetSimPortal provides an interface to network simulator in the mrnes package.
// mrnes does not import pces (to avoid circular imports).  However,
// code in pces can call a function in mrnes that returns a pointer
// to a structure that satisfies the NetSimPortal interface.
type NetSimPortal interface {
	HostCPU(string) string
	EnterNetwork(*evtm.EventManager, string, string, int, int, float64, any,
		any, evtm.EventHandlerFunction, any, evtm.EventHandlerFunction) any
}

// The TraceManager interface helps integrate use of the mrnes functionality for managing
// traces in the pces package
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

// pces pointers to mrnes implemenations of the NetworkPortal and TraceManager interfaces
var netportal *mrnes.NetworkPortal
var traceMgr TraceManager

var CmpPtnMapDict *CompPatternMapDict
var funcExecTimeTbl map[string]map[string]map[int]float64
var nameToSharedCfg map[string]any
var funcInstToSharedCfg map[GlobalFuncID]any

// A CmpPtnGraph is the run-time description of a CompPattern
type CmpPtnGraph struct {

	// every instance a function has a string 'label', used here to index to
	// data structure that describes it and the connections it has with other funcs
	Nodes map[string]*CmpPtnGraphNode
}

// CmpPtnGraphEdge declares the possibility that a function with label srcLabel
// might send a message of type msgType to the function (in the same CPG) with label dstLabel
type CmpPtnGraphEdge struct {
	SrcLabel   string
	MsgType    string
	DstLabel   string
	MethodCode string
}

func (cpge *CmpPtnGraphEdge) EdgeStr() string {
	rtn := fmt.Sprintf("src %s, type %s, dst %s, method %s",
		cpge.SrcLabel, cpge.MsgType, cpge.DstLabel, cpge.MethodCode)
	return rtn
}

type ExtCmpPtnGraphEdge struct {
	SrcCP string
	DstCP string
	CPGE  CmpPtnGraphEdge
}

func CreateCmpPtnGraphEdge(srcLabel, msgType, dstLabel, methodCode string) *CmpPtnGraphEdge {
	cpge := &CmpPtnGraphEdge{SrcLabel: srcLabel, MsgType: msgType, DstLabel: dstLabel, MethodCode: methodCode}
	return cpge
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
func (cpg *CmpPtnGraph) addEdge(srcLabel, msgType, dstLabel, methodCode string) error {
	edge := &CmpPtnGraphEdge{SrcLabel: srcLabel, MsgType: msgType, DstLabel: dstLabel, MethodCode: methodCode}

	srcNode, present1 := cpg.Nodes[srcLabel]
	if !present1 {
		return fmt.Errorf("srcLabel offered to addEdge not recognized")
	}

	dstNode, present2 := cpg.Nodes[dstLabel]
	if !present2 {
		return fmt.Errorf("dstLabel offered to addEdge not recognized")
	}

	// add to srcNode outEdges, if needed, and if not initiation
	duplicated := false
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
		err := cpg.addEdge(edge.SrcLabel, edge.MsgType, edge.DstLabel, edge.MethodCode)
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
func buildCmpPtns(cpd *CompPatternDict, cpid *CPInitListDict, ssgl *SharedCfgGroupList) error {

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

	// after all the patterns have been built, create their edge tables
	buildAllEdgeTables(cpd)

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
			cpumodel := funcExecDesc.CPUModel
			pcktLen := funcExecDesc.PcktLen
			execTime := funcExecDesc.ExecTime

			// use the cpuType and pckLen attributes to flesh out the execution time table
			_, present := et[operation][cpumodel]
			if !present {
				et[operation][cpumodel] = make(map[int]float64)
			}

			et[operation][cpumodel][pcktLen] = execTime
		}
	}
	return et
}

// BuildExperimentCP is called from the module that creates and runs
// a simulation. Its inputs identify the names of input files, which it
// uses to assemble and initialize the model (and experiment) data structures.
// It returns a pointer to an EventManager data structure used to coordinate the
// execution of events in the simulation.
func BuildExperimentCP(syn map[string]string, useYAML bool, idCounter int, tm TraceManager) (*evtm.EventManager, error) {
	// syn is a map that binds pre-defined keys referring to input file types with file names
	// The keys are
	//	"cpInput"		- file describing comp patterns and functions
	//	"cpInitInput"	- file describing initializations for comp patterns and functions
	//	"funcExecInput"	- file describing function and device operation execution timing
	//	"mapInput"		- file describing the mapping of functions to hosts

	// "qksim" if present means to tell the network portal to use quick simulation

	// call GetExperimentCPDicts to do the heavy lifting of extracting data structures
	// (typically maps) designed for serialization/deserialization,  and assign those maps to variables
	// we'll use to re-represent this information in structures optimized for run-time use
	cpd, cpid, ssgl, fel, cpmd := GetExperimentCPDicts(syn)

	// get a pointer to a mrns NetworkPortal

	netportal = mrnes.CreateNetworkPortal()
	_, use := syn["qksim"]
	netportal.SetQkNetSim(use)

	// panic if any one of these dictionaries could not be built
	if (cpd == nil) || (cpid == nil) || (fel == nil) || (cpmd == nil) {
		panic("empty dictionary")
	}

	// N.B. ssgl may be empty if there are no functions with shared cfg
	NumIDs = idCounter
	traceMgr = tm

	// remember the mapping of functions to host
	CmpPtnMapDict = cpmd

	// build the tables used to look up the execution time of comp pattern functions, and device operations
	funcExecTimeTbl = buildFuncExecTimeTbl(fel)

	buildSharedCfgMaps(ssgl, useYAML)

	// create the run-time representation of comp patterns, and initialize them
	CreateClassMethods()
	err := buildCmpPtns(cpd, cpid, ssgl)

	// check the coherence of the shared cfg groups
	if ssgl != nil {
		checkSharedCfgAssignment(ssgl)
	}

	// schedule the initiating events on self-initiating funcs
	evtMgr := evtm.New()
	schedInitEvts(evtMgr)
	return evtMgr, err
}

// NumIDs holds value that utility function used for generating unique integer ids on demand
var NumIDs int = 0

func nxtID() int {
	NumIDs += 1
	return NumIDs
}

// GetExperimentCPDicts accepts a map that holds the names of the input files used to define an experiment,
// creates internal representations of the information they hold, and returns those structs.
func GetExperimentCPDicts(syn map[string]string) (*CompPatternDict, *CPInitListDict,
	*SharedCfgGroupList, *FuncExecList, *CompPatternMapDict) {

	var cpd *CompPatternDict
	var cpid *CPInitListDict
	var fel *FuncExecList
	var cpmd *CompPatternMapDict
	var scgl *SharedCfgGroupList

	empty := make([]byte, 0)

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

	scgl = nil
	if len(syn["sharedCfg"]) > 0 {
		ext = path.Ext(syn["sharedCfg"])
		useYAML = (ext == ".yaml") || (ext == ".yml")

		scgl, err = ReadSharedCfgGroupList(syn["sharedCfg"], useYAML, empty)
		errs = append(errs, err)
	}

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

	return cpd, cpid, scgl, fel, cpmd
}

// buildSharedCfgMaps fills out two maps used to initialize funcs with shared cfg.
// The read-in list of shared cfg groups (if any) are examined
// to create the initial set up of the shared cfg, create one map that, given the name of the
// shared cfg group returns a pointer to that state, and another which given the (cmpPtnName, label)
// of a function that has shared cfg, a pointer to that state.
func buildSharedCfgMaps(ssgl *SharedCfgGroupList, useYAML bool) {

	// map index is the shared cfg group identity
	nameToSharedCfg = make(map[string]any)

	// map index is struct whose first member is name of a comp pattern, and the second one is the label within
	funcInstToSharedCfg = make(map[GlobalFuncID]any)

	// if there are no shared cfg groups we can leave now that the maps above are initialized to be empty
	if ssgl == nil {
		return
	}

	// the shared cfg groups are simply listed
	for _, ssg := range ssgl.Groups {

		// get a pointer to the class
		fc := FuncClasses[ssg.Class]

		// create the state for this group, what the maps will point to
		cfg := fc.CreateCfg(ssg.CfgStr, useYAML)

		// given the group name, get a pointer to the state structure
		nameToSharedCfg[ssg.Name] = cfg

		for _, gfid := range ssg.Instances {
			// given the (cmpPtnName, label) identity, get a pointer to the state structure
			funcInstToSharedCfg[gfid] = cfg
		}
	}
}

// checkSharedCfgAssignment ensures that every function instance in a
// shared cfg group has the same class
func checkSharedCfgAssignment(ssgl *SharedCfgGroupList) {

	// shared cfg groups in *ssgl are listed in unordered sequence
	for _, ssg := range ssgl.Groups {

		// check all the functions with shared cfg in the same group
		for _, gfid := range ssg.Instances {

			// pull out the function's comp pattern label
			ptnName := gfid.CmpPtnName
			label := gfid.Label

			// find representation of the comp pattern instance
			cpInst, present := CmpPtnInstByName[ptnName]
			if !present {
				panic(fmt.Errorf("comp pattern name %s from shared cfg group %s not found", ptnName, ssg.Name))
			}

			// find representation of the comp pattern's function
			cpf, present := cpInst.Funcs[label]
			if !present {
				panic(fmt.Errorf("function label %s from shared cfg group %s not found", label, ssg.Name))
			}

			// the class of the func instance needs to be the class of the shared cfg group
			if cpf.Class != ssg.Class {
				panic(fmt.Errorf("(%s,%s) from shared cfg group %s suffers class mismatch", ptnName, label, ssg.Name))
			}
		}
	}
}

func ReportStatistics() {
	// gather data by trace group
	tgData := make(map[string][]float64)

	for tgName, tg := range allTrackingGroups {
		_, present := tgData[tgName]
		if !present {
			tgData[tgName] = make([]float64, 0)
		}

		rec := tg.Finished
		if rec.n > 0 {
			tgData[tgName] = append(tgData[tgName], rec.samples...)
		}
	}

	var minv, maxv, mean, med, q25, q75 float64
	var qrt int
	for name, data := range tgData {
		sort.Float64s(data)
		num := len(data)
		sum := 0.0
		for _, v := range data {
			sum += v
		}
		mean = sum / float64(num)

		if num > 4 {
			medp := int(num / 2)
			minv = data[0]
			maxv = data[len(data)-1]
			if num%2 == 1 {
				med = data[medp+1]
			} else {
				med = (data[medp] + data[medp+1]) / 2
			}

			if num%4 == 0 {
				qrt = int(num / 4)
				q25 = (data[qrt-1] + data[qrt]) / 2
				q75 = (data[3*qrt] + data[3*qrt-1]) / 2
			} else if num%4 == 2 {
				q25 = data[int(num/4)]
				q75 = data[int(num/2)+int(num/4)]
			} else if num%4 == 1 {
				// means that the list length is odd, lengths from median to
				// endpoints are even
				mid := (num - 1) / 2
				qrt = int(mid / 2)
				q25 = (data[qrt] + data[qrt+1]) / 2
				q75 = (data[mid+qrt] + data[mid+qrt+1]) / 2
			} else {
				// list length is odd, lengths from median to endpoints are odd
				mid := (num - 1) / 2
				qrt = int(mid/2) + 1
				q25 = data[qrt]
				q75 = data[mid+qrt]
			}
		} else if num == 4 {
			minv = data[0]
			maxv = data[3]
			med = (data[1] + data[2]) / 2
			q25 = data[1]
			q75 = data[2]
		} else if num == 3 {
			minv = data[0]
			maxv = data[2]
			med = data[1]
			q25 = (data[0] + data[1]) / 2
			q75 = (data[2] + data[1]) / 2
		} else if num == 2 {
			minv = data[0]
			maxv = data[1]
			med = (data[1] + data[0]) / 2
			q25 = (data[0] + data[1]) / 4
			q75 = 3 * (data[0] + data[1]) / 4
		} else if num == 1 {
			minv = data[0]
			maxv = data[0]
			med = data[0]
			q25 = data[0]
			q75 = data[0]
		}
		fmt.Printf("Trace gathering group %s has spread %f, %f, %f %f, %f, %f\n", name, minv, q25, mean, med, q75, maxv)
	}
}
