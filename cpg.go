package pces

// file cpg.go holds structs, methods, and data structures related to the coordination of
// executing pces models through the APIs of computational pattern function instances

import (
	"errors"
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
	"github.com/iti/mrnes"
	"github.com/iti/rngstream"
	"gopkg.in/yaml.v3"
	"math"
	"strings"
	"sort"
)

// CompPattern functions, messages, and edges are described by structs in the desc package,
// and at runtime are read into pces.  Runtime data-structures (such as CmpPtnInst
// and CmpPtnMsg) are created from constructors that take desc structures as arguments.

// CmpPtnInstByName and CmpPtnInstByID take the the name (alt., id) of an instance of a CompPattern,
// and associate a pointer to its struct
var CmpPtnInstByName map[string]*CmpPtnInst = make(map[string]*CmpPtnInst)
var CmpPtnInstByID map[int]*CmpPtnInst = make(map[int]*CmpPtnInst)

type execRecord struct {
	cpID  int     // identity of comp pattern starting this execution
	src   string  // func initiating execution trace
	start float64 // time the trace started
}

type execSummary struct {
	n         int       // number of executions
	samples   []float64 // collection of samples
	completed int       // number of completions
	sum       float64   // sum of measured execution times of completed
	sum2      float64   // sum of squared execution times
	maxv      float64   // largest value seen
	minv      float64   // least value seen
}

// CmpPtnInst describes a particular instance of a CompPattern,
// built from information read in from CompPatternDesc struct and used at runtime
type CmpPtnInst struct {
	Name      string // this instance's particular name
	CpType    string
	ID        int                        // unique id
	Funcs     map[string]*CmpPtnFuncInst // use func label to get to func in that pattern with that label
	FuncsByGroup map[string][]*CmpPtnFuncInst // index is group name, list of functions in the CmpPtn belonging to that group
	Services  map[string]funcDesc        // map an offered service (e.g., "auth", "encrypt", "hash") to a function label
	Msgs      map[string]CompPatternMsg  // MsgType indexes msgs
	Rngs      *rngstream.RngStream
	Graph     *CmpPtnGraph                      // graph describing structure of funcs and edges
	Active    map[int]execRecord                // executions that are active now
	ActiveCnt map[int]int                       // number of instances of executions with common execID (>1 by branching)
	LostExec  map[int]evtm.EventHandlerFunction // call this handler when a packet for a given execID is lost
	Finished  map[string]execSummary            // summary of completed executions
}

// createCmpPtnInst is a constructor.  Inputs given by two structs from desc package.
// cpd describes the CompPattern, and cpid describes this instance's initialization parameters.
func createCmpPtnInst(ptnInstName string, cpd CompPattern, cpid CPInitList, evtMgr *evtm.EventManager) (*CmpPtnInst, error) {
	cpi := new(CmpPtnInst)

	// cpd.Edges is list of CmpPtnGraphEdge structs with attributes
	// SrcLabel, MsgType, DstLabel, the assumption being that
	// the labels are for funcs all in the CPI.
	// cpd.ExtEdges is a map whose index is the name of some comp pattern instance, with
	// attribute being a list of CmpPtnGraphEdges where the SrcLabel belongs to 'this'
	// CPI and the DstLabel (and MethodCode) belong to the CPI whose name is the index
	// the instance gets name on the input list
	cpi.Name = ptnInstName

	// get assuredly unique id
	cpi.ID = nxtID()

	// initialize slice of the func instances that make up the cpmPtnInst
	cpi.Funcs = make(map[string]*CmpPtnFuncInst)

	// initialize map of group names to functions in that group
	cpi.FuncsByGroup = make(map[string][]*CmpPtnFuncInst)

	// initialize slice of the service func instances that make up the cpmPtnInst
	cpi.Services = make(map[string]funcDesc)

	// initialize map that carries information about active execution threads
	cpi.Active = make(map[int]execRecord)

	// initialize map that carries number of active execs with same execid
	cpi.ActiveCnt = make(map[int]int)

	// initialize map that carries event scheduler to call if packet on execID is lost
	cpi.LostExec = make(map[int]evtm.EventHandlerFunction)

	// initialize map that carries information about completed executions
	cpi.Finished = make(map[string]execSummary)

	// make a representation of the CmpPtnFuncInst where funcs are nodes and edges are
	// labeled with message type
	var gerr error
	cpi.Graph, gerr = createCmpPtnGraph(&cpd)

	// enable access to this struct through its name, and through its id
	CmpPtnInstByName[cpi.Name] = cpi
	CmpPtnInstByID[cpi.ID] = cpi

	// The cpd structure has a list of desc descriptions of functions.
	// Call a constructor for each, depending on the func's execution type.
	// Save the created runtime representation of the function in the CmpPtnInst's list of funcs
	for _, funcDesc := range cpd.Funcs {
		if !validFuncClass(funcDesc.Class) {
			panic(fmt.Errorf("function class %s not recognized", funcDesc.Class))
		}

		df := createFuncInst(ptnInstName, cpi.ID, &funcDesc, cpid.Cfg[funcDesc.Label], cpid.UseYAML, evtMgr)
		cpi.Funcs[df.Label] = df
	}

	// save copies of all the messages for this CompPattern found in the initialization struct's list of messages
	cpi.Msgs = make(map[string]CompPatternMsg)
	for _, msg := range cpid.Msgs {
		cpi.Msgs[msg.MsgType] = msg
	}

	// create and save an rng
	cpi.Rngs = rngstream.New(cpi.Name)

	return cpi, gerr
}

func (cpi *CmpPtnInst) AddFuncToGroup(cpfi *CmpPtnFuncInst, groupName string) {
	_, present := cpi.FuncsByGroup[groupName]
	if !present {
		cpi.FuncsByGroup[groupName] = make([]*CmpPtnFuncInst, 0)
	}
	cpi.FuncsByGroup[groupName] = append(cpi.FuncsByGroup[groupName], cpfi)
}
 

// createSrvFuncLinks establishes the Services table for the computational patterns
func createSrvFuncLinks(cpd CompPattern) {
	cpi := CmpPtnInstByName[cpd.Name]
	for srvOp, srvDesc := range cpd.Services {
		// check whether we need to cite a different CP
		scpi := cpi
		if len(srvDesc.CP) > 0 {
			scpi = CmpPtnInstByName[srvDesc.CP]
		}

		// check that we know about the referenced function
		df, present := scpi.Funcs[srvDesc.Label]
		if !present {
			panic(fmt.Errorf("service function name confusion"))
		}

		// tag the function instance as being a service and save the reference to it
		df.IsService = true
		cpi.Services[srvOp] = srvDesc
	}
}

// buildAllEdgeTables goes through all the edges declared to the computational patterns
// and extracts from these the information needed to populate function instance data
// structures with what they need to recognize legimate messages and call the right methods
func buildAllEdgeTables(cpd *CompPatternDict) {
	// organize edges by comp pattern, inEdge, and outEdge, and whether x-CP
	cmpPtnEdges := make(map[string]map[string]map[string][]*ExtCmpPtnGraphEdge)
	for cpName := range cpd.Patterns {
		cmpPtnEdges[cpName] = make(map[string]map[string][]*ExtCmpPtnGraphEdge)
		cpi := CmpPtnInstByName[cpName]

		for funcLabel := range cpi.Funcs {
			cmpPtnEdges[cpName][funcLabel] = make(map[string][]*ExtCmpPtnGraphEdge)
			cmpPtnEdges[cpName][funcLabel]["InEdge"] = []*ExtCmpPtnGraphEdge{}
			cmpPtnEdges[cpName][funcLabel]["OutEdge"] = []*ExtCmpPtnGraphEdge{}
		}
	}

	// go through all edges and categorize each
	for cpName, cp := range cpd.Patterns {
		// go through the internal edges
		for _, edge := range cp.Edges {
			// note edge as an InEdge, transform to edgeStruct
			XEdge := new(ExtCmpPtnGraphEdge)
			XEdge.CPGE = edge
			XEdge.SrcCP = cpName
			XEdge.DstCP = cpName
			cmpPtnEdges[cpName][edge.DstLabel]["InEdge"] =
				append(cmpPtnEdges[cpName][edge.DstLabel]["InEdge"], XEdge)

			// if not an initiation edge note edge as an OutEdge
			if XEdge.CPGE.SrcLabel == XEdge.CPGE.DstLabel {
				continue
			}
			cmpPtnEdges[cpName][edge.SrcLabel]["OutEdge"] =
				append(cmpPtnEdges[cpName][edge.SrcLabel]["OutEdge"], XEdge)
		}

		// go through the external edges.
		for _, xedge := range cp.ExtEdges {
			XEdge := new(ExtCmpPtnGraphEdge)
			XEdge.SrcCP = cpName
			XEdge.DstCP = xedge.DstCP
			XEdge.CPGE = CmpPtnGraphEdge{SrcLabel: xedge.SrcLabel, MsgType: xedge.MsgType,
				DstLabel: xedge.DstLabel}

			// note xedge as an InEdge. The SrcLabel refers to a function in cpName.
			// a chgCP func is assumed to have (and performed) the transformation that identifies
			// the message type and dstLabel that will also be found in the edge
			cmpPtnEdges[xedge.DstCP][xedge.DstLabel]["InEdge"] =
				append(cmpPtnEdges[xedge.DstCP][xedge.DstLabel]["InEdge"], XEdge)

			// note xedge as an OutEdge.
			cmpPtnEdges[xedge.SrcCP][xedge.SrcLabel]["OutEdge"] =
				append(cmpPtnEdges[xedge.SrcCP][xedge.SrcLabel]["OutEdge"], XEdge)
		}

	}

	// now each comp function instance needs to have its outEdges slices created
	for cpName := range cpd.Patterns {
		cpi := CmpPtnInstByName[cpName]
		for fLabel, cpfi := range cpi.Funcs {
			for _, edge := range cmpPtnEdges[cpName][fLabel]["OutEdge"] {
				es := createEdgeStruct(cpfi.CPID, edge.CPGE.DstLabel, edge.CPGE.MsgType)
				_, present := cpfi.Msg2Idx[es.MsgType]
				if !present {
					cpfi.Msg2Idx[es.MsgType] = len(cpfi.OutEdges)
				} else {
					panic(fmt.Errorf("duplicated OutEdge specification"))
				}
				dstCPID := CmpPtnInstByName[edge.DstCP].ID
				es = createEdgeStruct(dstCPID, edge.CPGE.DstLabel, edge.CPGE.MsgType)
				cpfi.OutEdges = append(cpfi.OutEdges, es)
			}
		}
	}

	// validate the configurations (some of which depend on these edges)
	for cpName := range cpd.Patterns {
		cpi := CmpPtnInstByName[cpName]
		for _, cpfi := range cpi.Funcs {
			fc := FuncClasses[cpfi.Class]
			fc.ValidateCfg(cpfi)
		}
	}
}

// buildAllEdgeTables goes through all the edges declared to the computational patterns
// and extracts from these the information needed to populate function instance data
// structures with what they need to recognize legimate messages and call the right methods
func buildAllEdgeTablesOld(cpd *CompPatternDict) {
	// organize edges by comp pattern, inEdge, and outEdge, and whether x-CP
	cmpPtnEdges := make(map[string]map[string]map[string][]*ExtCmpPtnGraphEdge)
	for cpName := range cpd.Patterns {
		cmpPtnEdges[cpName] = make(map[string]map[string][]*ExtCmpPtnGraphEdge)
		cpi := CmpPtnInstByName[cpName]

		for funcLabel := range cpi.Funcs {
			cmpPtnEdges[cpName][funcLabel] = make(map[string][]*ExtCmpPtnGraphEdge)
			cmpPtnEdges[cpName][funcLabel]["InEdge"] = []*ExtCmpPtnGraphEdge{}
			cmpPtnEdges[cpName][funcLabel]["OutEdge"] = []*ExtCmpPtnGraphEdge{}
			// cmpPtnEdges[cpName][funcLabel]["ExtInEdge"] = []*ExtCmpPtnGraphEdge{}
			// cmpPtnEdges[cpName][funcLabel]["ExtOutEdge"] = []*ExtCmpPtnGraphEdge{}
		}
	}

	// go through all edges and categorize each
	for cpName, cp := range cpd.Patterns {
		// go through the internal edges
		for _, edge := range cp.Edges {
			// note edge as an InEdge, transform to edgeStruct
			XEdge := new(ExtCmpPtnGraphEdge)
			XEdge.CPGE = edge
			cmpPtnEdges[cpName][edge.DstLabel]["InEdge"] =
				append(cmpPtnEdges[cpName][edge.DstLabel]["InEdge"], XEdge)

			// if not an initiation edge note edge as an OutEdge
			if XEdge.CPGE.SrcLabel == XEdge.CPGE.DstLabel {
				continue
			}
			cmpPtnEdges[cpName][edge.SrcLabel]["OutEdge"] =
				append(cmpPtnEdges[cpName][edge.SrcLabel]["OutEdge"], XEdge)
		}

		// go through the external edges.
		for _, xedge := range cp.ExtEdges {
			XEdge := new(ExtCmpPtnGraphEdge)
			XEdge.SrcCP = cpName
			XEdge.DstCP = xedge.DstCP
			XEdge.CPGE = CmpPtnGraphEdge{SrcLabel: xedge.SrcLabel, MsgType: xedge.MsgType,
				DstLabel: xedge.DstLabel}

			// note xedge as an InEdge. The SrcLabel refers to a function in cpName.
			// a chgCP func is assumed to have (and performed) the transformation that identifies
			// the message type and dstLabel that will also be found in the edge
			cmpPtnEdges[xedge.DstCP][xedge.DstLabel]["ExtInEdge"] =
				append(cmpPtnEdges[xedge.DstCP][xedge.DstLabel]["ExtInEdge"], XEdge)

			// note xedge as an InEdge. The SrcLabel refers to a function in cpName.
			// a chgCP func is assumed to have (and performed) the transformation that identifies
			// the message type and dstLabel that will also be found in the edge
			cmpPtnEdges[xedge.DstCP][xedge.DstLabel]["ExtInEdge"] =
				append(cmpPtnEdges[xedge.DstCP][xedge.DstLabel]["ExtInEdge"], XEdge)

		}

	}

	// now each comp function instance needs to have its outEdges slices created
	for cpName := range cpd.Patterns {
		cpi := CmpPtnInstByName[cpName]
		for fLabel, cpfi := range cpi.Funcs {
			for _, edge := range cmpPtnEdges[cpName][fLabel]["OutEdge"] {
				es := createEdgeStruct(cpfi.CPID, edge.CPGE.DstLabel, edge.CPGE.MsgType)
				_, present := cpfi.Msg2Idx[es.MsgType]
				if !present {
					cpfi.Msg2Idx[es.MsgType] = len(cpfi.OutEdges)
				} else {
					panic(fmt.Errorf("duplicated OutEdge specification"))
				}
				cpfi.OutEdges = append(cpfi.OutEdges, es)
			}

			// N.B. one can declare external out edges, but one does not require them
			// for inter-CP communication...the transfer function will do that when used
			for _, edge := range cmpPtnEdges[cpName][fLabel]["ExtOutEdge"] {
				dstCPID := CmpPtnInstByName[edge.DstCP].ID
				es := createEdgeStruct(dstCPID, edge.CPGE.DstLabel, edge.CPGE.MsgType)
				_, present := cpfi.Msg2Idx[es.MsgType]
				if !present {
					cpfi.Msg2Idx[es.MsgType] = len(cpfi.OutEdges)
				} else {
					panic(fmt.Errorf("duplicated Msg2Idx specification"))
				}
				cpfi.OutEdges = append(cpfi.OutEdges, es)
			}
		}
	}

	// validate the configurations (some of which depend on these edges)
	for cpName := range cpd.Patterns {
		cpi := CmpPtnInstByName[cpName]
		for _, cpfi := range cpi.Funcs {
			fc := FuncClasses[cpfi.Class]
			fc.ValidateCfg(cpfi)
		}
	}
}


type trackingGroup struct {
	Name      string
	Active    map[int]execRecord
	Finished  execSummary
	ActiveCnt int
}

var allTrackingGroups map[string]*trackingGroup = make(map[string]*trackingGroup)
var execIDToTG map[int]*trackingGroup = make(map[int]*trackingGroup)

func createTrackingGroup(name string) *trackingGroup {
	tg := new(trackingGroup)
	tg.Name = name
	tg.Active = make(map[int]execRecord)
	tg.Finished.samples = []float64{}
	allTrackingGroups[name] = tg
	return tg
}

// startRecExec records the name of the initialiating func, and starting
// time of an execution trace in the named tracking group
func startRecExec(tgName string, execID, cpID int, funcName string, time float64) {
	// ensure we don't start the same execID more than once
	_, present := execIDToTG[execID]
	if present {
		panic(fmt.Errorf("attempt to start already started tracking on execID %d", execID))
	}

	// create a tracking group if needed
	tg, present := allTrackingGroups[tgName]
	if !present {
		tg = createTrackingGroup(tgName)
	}
	execIDToTG[execID] = tg

	activeRec := execRecord{cpID: cpID, src: funcName, start: time}

	tg.Active[execID] = activeRec
}

// EndRecExec computes the completed execution time of the execution identified,
// given the ending time, incorporates its statistics into the CmpPtnInst
// statistics summary
func EndRecExec(execID int, time float64) float64 {
	// make sure we have a tracking group for this execID
	tg, present := execIDToTG[execID]
	if !present {
		panic(fmt.Errorf("failure to find tracking group defined for execID %d", execID))
	}

	rtn := time - tg.Active[execID].start
	delete(tg.Active, execID)

	tg.Finished.n += 1
	tg.Finished.samples = append(tg.Finished.samples, rtn)

	tg.Finished.completed += 1
	tg.Finished.sum += rtn
	tg.Finished.sum2 += rtn * rtn
	tg.Finished.maxv = math.Max(tg.Finished.maxv, rtn)
	tg.Finished.minv = math.Min(tg.Finished.minv, rtn)
	return rtn
}

func (cpi *CmpPtnInst) cleanUp() {
	for execID := range cpi.Active {
		delete(cpi.Active, execID)
	}
}

// ExecReport reports delay times of completed cmpPtnInst executions
func (cpi *CmpPtnInst) ExecReport() []float64 {
	cpi.cleanUp()

	for _, rec := range cpi.Finished {
		if rec.n > 0 {
			return rec.samples
		} else {
			continue
		}
	}
	return []float64{}
}

type CPEndpt struct {
	CPID  int
	Label string
}

type CPEntry struct {
	CPID    int
	Label   string
	MsgType string
}

// EndPtFuncs carries information on a CmpPtnMsg about where the execution thread started,
// and where it ultimately is headed
type EndPtFuncs struct {
	Srt CPEndpt
	End CPEndpt
}

// A CmpPtnMsg struct describes a message going from one CompPattern function to another.
// It carries ancillary information about the message that is included for referencing.
type CmpPtnMsg struct {
	ExecID    int    // initialize when with an initating comp pattern message.  Carried by every resulting message.
	FlowID    int    // identity of flow when message involves flows
	ClassID   int    // identity of transportation class when message hits network
	MeasureID int    // when carrying a measure, the identity of that measure
	PrevCPID  int    // ID of the comp pattern through which the message most recently passed
	PrevLabel string // label of the func through which the message most recently passed

	XCPID  int    // cross-CP id
	XLabel string // cross-CP label
	CPID   int    // identity of the destination comp pattern
	Label  string // label of the destination function

	MsgType string // describes function of message
	MsgLen  int    // number of bytes

	RtnCPID    int    // used on service calls
	RtnLabel   string // used on service calls
	RtnMsgType string

	PcktLen    int     // parameter impacting execution time
	Rate       float64 // when non-zero, a rate limiting attribute that might used, e.g., in modeling IO
	FlowState  string  // "srt", "end", "chg"
	Start      bool    // start the timer
	StartTime  float64 // when the timer started
	NetLatency float64
	NetBndwdth float64
	NetPrLoss  float64
	Payload    any // free for "something else" to carry along and be used in decision logic
}

func (cpm *CmpPtnMsg) Populate(execID, flowID, classID int, rate float64, msgLen int, flowState string) {
	cpm.ExecID = execID
	cpm.FlowID = flowID
	cpm.ClassID = classID
	cpm.Rate = rate
	cpm.FlowState = flowState
}

type CmpPtnMsgTrace struct {
	ExecID int // initialize when with an initating comp pattern message.  Carried by every resulting message.

	NxtLabel string // string label of the function the message next visits

	MsgType   string // describes function of message
	MsgLen    int    // number of bytes
	PcktLen   int    // parameter impacting execution time
	FlowState string // "srt", "end", "chg"
}

func (cpm *CmpPtnMsg) Serialize() string {
	var bytes []byte
	var merr error

	revcpm := new(CmpPtnMsgTrace)
	revcpm.ExecID = cpm.ExecID
	revcpm.MsgType = cpm.MsgType
	revcpm.MsgLen = cpm.MsgLen
	revcpm.FlowState = cpm.FlowState

	bytes, merr = yaml.Marshal(*revcpm)

	if merr != nil {
		panic(fmt.Errorf("error serializing CmpPtnMsgTrace"))
	}

	return string(bytes[:])
}

// CarriesPckt indicates whether the message conveys information about a packet or a flow
func (cpm *CmpPtnMsg) CarriesPckt() bool {
	return (cpm.MsgLen > 0 && cpm.PcktLen > 0)
}

// CmpPtnFuncInstHost returns the name of the host to which the CmpPtnFuncInst given as argument is mapped.
func CmpPtnFuncInstHost(cpfi *CmpPtnFuncInst) string {
	cpfLabel := cpfi.funcLabel()
	cpfCmpPtn := cpfi.funcCmpPtn()

	return CmpPtnMapDict.Map[cpfCmpPtn].FuncMap[cpfLabel]
}

func isNOP(op string) bool {
	return (op == "noop" || op == "NOOP" || op == "NOP" || op == "no-op" || op == "nop")
}

// HostFuncExecTime returns the execution time for the operation given
// on the endpoint to which the cpfi is mapped
func HostFuncExecTime(cpfi *CmpPtnFuncInst, op string, msg *CmpPtnMsg) float64 {
	hostLabel := CmpPtnFuncHost(cpfi)
	cpumodel := netportal.EndptDevModel(hostLabel, "")
	return funcExecTime(cpumodel, op, msg)
}

// AccelFuncExecTime returns the execution time for the operation given on
// the accelerator model given as input
func AccelFuncExecTime(cpfi *CmpPtnFuncInst, accelname, op string, msg *CmpPtnMsg) float64 {
	hostLabel := CmpPtnFuncHost(cpfi)
	accelmodel := netportal.EndptDevModel(hostLabel, accelname)
	return funcExecTime(accelmodel, op, msg)
}

// funcExecTime returns the increase in execution time resulting from executing the
// CmpPtnFuncInst offered as argument, to the message also offered as argument.
// If the pcktlen of the message does not exactly match the pcktlen parameter of a func timing entry,
// an interpolation or extrapolation of existing entries is performed.
func funcExecTime(model string, op string, msg *CmpPtnMsg) float64 {
	// get the parameters needed for the func execution time lookup
	if isNOP(op) {
		return 0.0
	}

	// if we don't have an entry for this function type, complain
	_, present := funcExecTimeTbl[op]
	if !present {
		panic(fmt.Errorf("no function timing look up for operation %s", op))
	}

	// if we don't have an entry for the named CPU for this function type, throw an error
	lenMap, here := funcExecTimeTbl[op][model]
	if !here || lenMap == nil {
		panic(fmt.Errorf("no function timing look for operation %s on cpu model %s", op, model))
	}

	// if msg is nil return the timing from the first entry in the map
	if msg == nil {
		keys := []int{}
		for key := range lenMap {
			keys = append(keys,key)
		}
		sort.Ints(keys)
		return lenMap[keys[0]]
	}

	// lenMap is map[int]string associating an execution time for a packet with the stated length,
	// so long as we have that length
	timing, present2 := lenMap[msg.PcktLen]
	if present2 {
		return timing // we have the length, so just return the timing
	}

	// length not present so estimate from data we do have about this function type and CPU.
	// If at least two measurements are present create a least-squares model and interpolate or
	// extrapolate to the argument pcklen.  If only one measurement is present compute the
	// 'time per byte' from that measurement and extrapolate to the message pcktlen.
	type XYPoint struct {
		x float64
		y float64
	}

	points := []XYPoint{}

	for pcktLen, timing := range lenMap {
		points = append(points, XYPoint{x: float64(pcktLen), y: timing})
	}

	// if there is only one point, extrapolate from time per unit length
	if len(points) == 1 {
		timePerUnit := points[0].y / points[0].x

		return float64(msg.PcktLen) * timePerUnit
	}

	// do a linear regression on the others
	sumX := float64(0.0)
	sumX2 := float64(0.0)
	sumY := float64(0.0)
	sumY2 := float64(0.0)
	sumXY := float64(0.0)

	for _, point := range points {
		sumX += point.x
		sumX2 += point.x * point.x
		sumY += point.y
		sumY2 += point.y * point.y
		sumXY += point.x * point.y
	}
	N := float64(len(points))
	m := (N*sumXY - sumX*sumY) / (N*sumX2 - sumX*sumX)
	b := (sumY - m*sumX) / N

	return float64(msg.PcktLen)*m + b
}

// EnterFunc is an event-handling routine, scheduled by an evtm.EventManager to execute and simulate the results of
// a message arrival to a CmpPtnInst function.  The particular CmpPtnInst and particular message
// are given as arguments to the function.  A type-unspecified return is provided.
func EnterFunc(evtMgr *evtm.EventManager, cpFunc any, cpMsg any) any {
	// extract the CmpPtnFuncInst and CmpPtnMsg involved in this event
	cpfi := cpFunc.(*CmpPtnFuncInst)

	// the choice of cpfi was made in ExitFunc by looking up
	// the function in the CmpPtnInst list of functions (selected by
	// the CPID on the outbound message), indexed by
	// the Label, also found on the outbound message.
	//   When this function is not starting an execution thread,
	// the method code selected to govern the response of this function
	// to the input msg (and message type) depends on the previous CPID, Label,
	// and MsgType.  The method code indexes into a table that looks up
	// a function to call to handle the input.

	// startFlag set to true if the value passed in as cpMsg is not empty and is not a pointer
	// to a CmpPtnMsg.   Means that the function call is of a start function, and
	// cpMsg is a *string, deferenced to give a method code
	//
	var stdFunc bool
	methodCode := "default"
	var cpm *CmpPtnMsg
	var msgType string

	// methodCode set to the Msg2MC code if cpm is a *MsgType and the message type is known to Msg2MC, else
	// methodCode set to Msg2MC["*"] if the presented msgType is not known to Msg2MC, but "*" is in the Msg2MC table
	// methodCode set to *cpMsg when it points to a string, interpreted
	// methodCode set to "default" otherwise
	if cpMsg != nil {

		// get the message type from the message, if it is a message, or
		// as the argument if non-nil, non-message argument (which must then be a *string)
		cpm, stdFunc = cpMsg.(*CmpPtnMsg)
		if stdFunc {
			msgType = cpm.MsgType
		} else {
			msgType = *cpMsg.(*string)
		}

		if len(msgType) > 0 && len(cpfi.Msg2MC) > 0 && len(cpfi.Msg2MC[msgType]) > 0 {
			methodCode = cpfi.Msg2MC[msgType]
		} else if len(msgType) > 0 && len(cpfi.Msg2MC) > 0 && len(cpfi.Msg2MC["*"]) > 0 {
			methodCode = cpfi.Msg2MC["*"]
		} else {
			methodCode = "default"
		}
	}
	if cpm != nil {
		AddCPTrace(TraceMgr, cpfi.Trace, evtMgr.CurrentTime(), cpm.ExecID, cpfi.ID, FullFuncName(cpfi,"EnterFunc"), cpm)
	} 

	methods, present := cpfi.RespMethods[methodCode]
	if !present {
		// methods might have been put in by user extension after the function's own setup
		classMethods, classpresent := ClassMethods[cpfi.Class][methodCode]
		if !classpresent {
			panic(fmt.Errorf("expected class %s to have response methods for method code %s", cpfi.Class, methodCode))
		}
		methods = &classMethods
		cpfi.RespMethods[methodCode] = methods
	}
	
	methods.Start(evtMgr, cpfi, methodCode, cpm)
	return nil
}

// EmptyInitFunc exists to detect when there is actually an initialization event handler
// (by having a 'emptyInitFunc' below be re-written to point to something else
func EmptyInitFunc(evtMgr *evtm.EventManager, cpFunc any, cpMsg any) any {
	return nil
}

var emptyInitFunc evtm.EventHandlerFunction = EmptyInitFunc

// EmptyEnterFunc exists to fill in the ClassMethods table
func EmptyEnterFunc(evtMgr *evtm.EventManager, cpti *CmpPtnFuncInst, data string, cpm *CmpPtnMsg) {
}

var emptyEnterFunc StartMethod = EmptyEnterFunc

// ExitFunc is an event handling routine that implements the scheduling of messages which result from the completed
// (simulated) execution of a CmpPtnFuncInst.   The CmpPtnFuncInst and the message that triggered the execution
// are given as arguments to ExitFunc.   This routine calls the CmpPtnFuncInst's function that computes
// the effect of doing the simulation, a routine which (by the CmpPtnFuncInst interface definition) returns a slice
// of CmpPtnMsgs which are then pushed further along the CompPattern chain.
func ExitFunc(evtMgr *evtm.EventManager, cpFunc any, cpMsg any) any {
	// extract the CmpPtnFuncInst and CmpPtnMsg involved in this event
	cpfi := cpFunc.(*CmpPtnFuncInst)
	cpm := cpMsg.(*CmpPtnMsg)

	// get the response(s), if any.  Note that result is a slice of CmpPtnMsgs.
	msgs := cpfi.funcResp(cpm.ExecID)

	// note exit from function
	AddCPTrace(TraceMgr, cpfi.Trace, evtMgr.CurrentTime(), cpm.ExecID, cpfi.ID, FullFuncName(cpfi,"ExitFunc"), cpm)

	// a problem if there are no messages, because the controled end of a thread
	// is supposed always to be a "finish" function
	if len(msgs) == 0 {
		print("unexpected ExitFunc call with no messages")
		return nil
	}

	// get a pointer to the comp pattern the function being exited belongs to
	cpi := CmpPtnInstByName[cpfi.funcCmpPtn()]

	// treat each msg individually
	for _, msg := range msgs {

		// allow for possibility that the next comp pattern is different, noticed
		xcpi := cpi
		if msg.CPID != cpfi.CPID {
			xcpi = CmpPtnInstByID[msg.CPID]
		}

		// save the previous CPID and Label
		msg.PrevCPID = cpfi.CPID
		msg.PrevLabel = cpfi.Label

		// the processing done in response to receipt of this message
		// depends on the value for Label and MsgType carried on the message,
		// and in the case of a communication between Funcs in the same CmpPtn.
		// determine whether destination is on processor or off processor
		nxtf, present := xcpi.Funcs[msg.Label]
		if present {
			dstHost := CmpPtnMapDict.Map[xcpi.Name].FuncMap[nxtf.Label]
			if strings.Contains(dstHost, ",") {
				pieces := strings.Split(dstHost, ",")
				dstHost = pieces[0]
			}

			// Staying on the host means scheduling w/o delay the arrival at the next func
			// through EnterFunc
			if cpfi.Host == dstHost {
				evtMgr.Schedule(nxtf, msg, EnterFunc, vrtime.SecondsToTime(0.0))
			} else {

				// to get to the dstHost we need to go through the network
				isPckt := msg.CarriesPckt()

				connDesc := new(mrnes.ConnDesc)
				if isPckt {
					connDesc.Type = mrnes.DiscreteConn
				} else {
					connDesc.Type = mrnes.FlowConn
				}

				if netportal.QkNetSim {
					connDesc.Latency = mrnes.Place
				} else {
					connDesc.Latency = mrnes.Simulate
				}

				switch cpm.FlowState {
				case "srt":
					connDesc.Action = mrnes.Srt
				case "end":
					connDesc.Action = mrnes.End
				case "chg":
					connDesc.Action = mrnes.Chg
				default:
					connDesc.Action = mrnes.None
				}

				IDs := mrnes.NetMsgIDs{ExecID: msg.ExecID, FlowID: msg.FlowID, ClassID: msg.ClassID}

				// indicate where the returning event is to be delivered
				rtnDesc := new(mrnes.RtnDesc)
				rtnDesc.Cxt = nxtf
				rtnDesc.EvtHdlr = ReEnter

				// indicate what to do if there is a packet loss
				lossDesc := new(mrnes.RtnDesc)
				lossDesc.Cxt = cpfi
				lossDesc.EvtHdlr = LostCmpPtnMsg

				rtns := mrnes.RtnDescs{Rtn: rtnDesc, Src: nil, Dst: nil, Loss: lossDesc}

				netportal.EnterNetwork(evtMgr, cpfi.Host, dstHost, msg.MsgLen, connDesc, IDs, rtns, msg.Rate, msg)
			}
		} else {
			panic(errors.New("exit function fails to find next function"))
		}
	}
	return nil
}

func ReEnter(evtMgr *evtm.EventManager, cpFunc any, rtnmsg any) any {
	// msg is of type *mrnes.RtnMsgStruct
	rtnMsg := rtnmsg.(*mrnes.RtnMsgStruct)
	msg := rtnMsg.Msg.(*CmpPtnMsg)
	msg.NetLatency = rtnMsg.Latency
	msg.NetBndwdth = rtnMsg.Rate
	msg.NetPrLoss = rtnMsg.PrLoss

	evtMgr.Schedule(cpFunc, msg, EnterFunc, vrtime.SecondsToTime(0.0))
	return nil
}

// LostCmpPtnMsg is scheduled from the mrnes side to report the loss of a comp pattern message
func LostCmpPtnMsg(evtMgr *evtm.EventManager, context any, msg any) any {
	cpMsg := msg.(*CmpPtnMsg)

	// cpMmsg.CP is the name of pattern
	execID := cpMsg.ExecID

	// look up a description of the comp pattern
	cpi := CmpPtnInstByID[cpMsg.CPID]

	cpi.ActiveCnt[execID] -= 1

	// if no other msgs active for this execID report loss
	if cpi.ActiveCnt[execID] == 0 {
		fmt.Printf("Comp Pattern %s lost message for execution id %d\n", cpi.Name, execID)
		return nil
	}
	return nil
}

// numExecThreads is used to place a unique integer code on every newly created initiation message
var numExecThreads int = 1
