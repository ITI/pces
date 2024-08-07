package pces

// file cpg.go holds structs, methods, and data structures related to the coordination of
// executing pces models through the APIs of computational pattern function instances

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
	"github.com/iti/mrnes"
	"github.com/iti/rngstream"
	"math"
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
	name      string // this instance's particular name
	cpType    string
	ID        int                        // unique id
	funcs     map[string]*CmpPtnFuncInst // use func label to get to func in that pattern with that label
	msgs      map[string]CompPatternMsg  // MsgType indexes msgs
	Rngs      *rngstream.RngStream
	Graph     *CmpPtnGraph                      // graph describing structure of funcs and edges
	active    map[int]execRecord                // executions that are active now
	activeCnt map[int]int                       // number of instances of executions with common execID (>1 by branching)
	lostExec  map[int]evtm.EventHandlerFunction // call this handler when a packet for a given execID is lost
	finished  map[string]execSummary            // summary of completed executions
}

// createCmpPtnInst is a constructor.  Inputs given by two structs from desc package.
// cpd describes the CompPattern, and cpid describes this instance's initialization parameters.
func createCmpPtnInst(ptnInstName string, cpd CompPattern, cpid CPInitList) (*CmpPtnInst, error) {
	cpi := new(CmpPtnInst)

	// cpd.Edges is list of CmpPtnGraphEdge structs with attributes
	// SrcLabel, MsgType, DstLabel, MethodCode, the assumption being that
	// the labels are for funcs all in the CPI.
	// cpd.ExtEdges is a map whose index is the name of some comp pattern instance, with
	// attribute being a list of CmpPtnGraphEdges where the SrcLabel belongs to 'this'
	// CPI and the DstLabel (and MethodCode) belong to the CPI whose name is the index

	// the instance gets name on the input list
	cpi.name = ptnInstName

	// get assuredly unique id
	cpi.ID = nxtID()

	// initialize slice of the func instances that make up the cpmPtnInst
	cpi.funcs = make(map[string]*CmpPtnFuncInst)

	// initialize map that carries information about active execution threads
	cpi.active = make(map[int]execRecord)

	// initialize map that carries number of active execs with same execid
	cpi.activeCnt = make(map[int]int)

	// initialize map that carries event scheduler to call if packet on execID is lost
	cpi.lostExec = make(map[int]evtm.EventHandlerFunction)

	// initialize map that carries information about completed executions
	cpi.finished = make(map[string]execSummary)

	// make a representation of the CmpPtnFuncInst where funcs are nodes and edges are
	// labeled with message type
	var gerr error
	cpi.Graph, gerr = createCmpPtnGraph(&cpd)

	// enable access to this struct through its name, and through its id
	CmpPtnInstByName[cpi.name] = cpi
	CmpPtnInstByID[cpi.ID] = cpi

	// The cpd structure has a list of desc descriptions of functions.
	// Call a constructor for each, depending on the func's execution type.
	// Save the created runtime representation of the function in the CmpPtnInst's list of funcs
	for _, funcDesc := range cpd.Funcs {
		if !validFuncClass(funcDesc.Class) {
			panic(fmt.Errorf("function class %s not recognized", funcDesc.Class))
		}

		df := createFuncInst(ptnInstName, cpi.ID, &funcDesc, cpid.Cfg[funcDesc.Label], cpid.UseYAML)
		cpi.funcs[df.Label] = df
	}

	// save copies of all the messages for this CompPattern found in the initialization struct's list of messages
	cpi.msgs = make(map[string]CompPatternMsg)
	for _, msg := range cpid.Msgs {
		cpi.msgs[msg.MsgType] = msg
	}

	// create and save an rng
	cpi.Rngs = rngstream.New(cpi.name)

	return cpi, gerr
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

		for funcLabel := range cpi.funcs {
			cmpPtnEdges[cpName][funcLabel] = make(map[string][]*ExtCmpPtnGraphEdge)
			cmpPtnEdges[cpName][funcLabel]["InEdge"] = []*ExtCmpPtnGraphEdge{}
			cmpPtnEdges[cpName][funcLabel]["OutEdge"] = []*ExtCmpPtnGraphEdge{}
			cmpPtnEdges[cpName][funcLabel]["ExtInEdge"] = []*ExtCmpPtnGraphEdge{}
			cmpPtnEdges[cpName][funcLabel]["ExtOutEdge"] = []*ExtCmpPtnGraphEdge{}
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

		// go through the external edges. Recall that type of cp.ExtEdges is map[string][]XCPEdge
		for _, xList := range cp.ExtEdges {
			for _, xedge := range xList {
				XEdge := new(ExtCmpPtnGraphEdge)
				XEdge.SrcCP = cpName
				XEdge.DstCP = xedge.DstCP
				XEdge.CPGE = CmpPtnGraphEdge{SrcLabel: xedge.SrcLabel, MsgType: xedge.MsgType,
					DstLabel: xedge.DstLabel, MethodCode: xedge.MethodCode}

				// note xedge as an InEdge. The SrcLabel refers to a function in cpName.
				// a chgCP func is assumed to have (and performed) the transformation that identifies
				// the message type and dstLabel that will also be found in the edge
				cmpPtnEdges[xedge.DstCP][xedge.DstLabel]["ExtInEdge"] =
					append(cmpPtnEdges[xedge.DstCP][xedge.DstLabel]["ExtInEdge"], XEdge)

				cmpPtnEdges[xedge.SrcCP][xedge.SrcLabel]["ExtOutEdge"] =
					append(cmpPtnEdges[xedge.SrcCP][xedge.SrcLabel]["ExtInEdge"], XEdge)
			}
		}
	}

	// now each comp function instance needs to have its inEdgeMethodCode and outEdges slices
	// created
	for cpName := range cpd.Patterns {
		cpi := CmpPtnInstByName[cpName]
		for fLabel, cpfi := range cpi.funcs {
			for _, edge := range cmpPtnEdges[cpName][fLabel]["InEdge"] {
				es := createEdgeStruct(cpfi.CPID, edge.CPGE.SrcLabel, edge.CPGE.MsgType)
				cpfi.inEdgeMethodCode[es] = edge.CPGE.MethodCode
			}

			for _, edge := range cmpPtnEdges[cpName][fLabel]["OutEdge"] {
				es := createEdgeStruct(cpfi.CPID, edge.CPGE.DstLabel, edge.CPGE.MsgType)
				cpfi.outEdges = append(cpfi.outEdges, es)
			}

			for _, edge := range cmpPtnEdges[cpName][fLabel]["ExtOutEdge"] {
				dstCPID := CmpPtnInstByName[edge.DstCP].ID
				es := createEdgeStruct(dstCPID, edge.CPGE.DstLabel, edge.CPGE.MsgType)
				cpfi.outEdges = append(cpfi.outEdges, es)
			}

			for _, edge := range cmpPtnEdges[cpName][fLabel]["ExtInEdge"] {
				srcCPID := CmpPtnInstByName[edge.SrcCP].ID
				es := createEdgeStruct(srcCPID, edge.CPGE.SrcLabel, edge.CPGE.MsgType)
				cpfi.inEdgeMethodCode[es] = edge.CPGE.MethodCode
			}
		}
	}

	// validate the configurations (some of which depend on these edges)
	for cpName := range cpd.Patterns {
		cpi := CmpPtnInstByName[cpName]
		for _, cpfi := range cpi.funcs {
			fc := FuncClasses[cpfi.class]
			fc.ValidateCfg(cpfi)
		}
	}
}

type trackingGroup struct {
	name      string
	active    map[int]execRecord
	finished  execSummary
	activeCnt int
}

var allTrackingGroups map[string]*trackingGroup = make(map[string]*trackingGroup)
var execIDToTG map[int]*trackingGroup = make(map[int]*trackingGroup)

func createTrackingGroup(name string) *trackingGroup {
	tg := new(trackingGroup)
	tg.name = name
	tg.active = make(map[int]execRecord)
	tg.finished.samples = []float64{}
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
	tg.active[execID] = activeRec
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

	rtn := time - tg.active[execID].start
	delete(tg.active, execID)

	tg.finished.n += 1
	tg.finished.samples = append(tg.finished.samples, rtn)

	tg.finished.completed += 1
	tg.finished.sum += rtn
	tg.finished.sum2 += rtn * rtn
	tg.finished.maxv = math.Max(tg.finished.maxv, rtn)
	tg.finished.minv = math.Min(tg.finished.minv, rtn)

	return rtn
}

func (cpi *CmpPtnInst) cleanUp() {
	for execID := range cpi.active {
		delete(cpi.active, execID)
	}
}

// ExecReport reports delay times of completed cmpPtnInst executions
func (cpi *CmpPtnInst) ExecReport() []float64 {
	cpi.cleanUp()

	for _, rec := range cpi.finished {
		if rec.n > 0 {
			return rec.samples
		} else {
			continue
		}
	}
	return []float64{}
}

// EndPtFuncs carries information on a CmpPtnMsg about where the execution thread started,
// and where it ultimately is headed
type EndPtFuncs struct {
	SrtLabel string
	SrtCPID int
	EndLabel string
	EndCPID int
}

// A CmpPtnMsg struct describes a message going from one CompPattern function to another.
// It carries ancillary information about the message that is included for referencing.
type CmpPtnMsg struct {
	ExecID   int     // initialize when with an initating comp pattern message.  Carried by every resulting message.
	prevCPID int     // ID of the comp pattern through which the message most recently passed
	prevLabel string // label of the func through which the message most recently passed
	nxtCPID int
	nxtLabel string

	CmpHdr EndPtFuncs

	MsgType string  // describes function of message
	MsgLen  int     // number of bytes
	PcktLen int     // parameter impacting execution time
	Rate    float64 // when non-zero, a rate limiting attribute that might used, e.g., in modeling IO
	Start   bool    // start the timer
	Payload any     // free for "something else" to carry along and be used in decision logic
}

// carriesPckt indicates whether the message conveys information about a packet or a flow
func (cpm *CmpPtnMsg) carriesPckt() bool {
	return (cpm.MsgLen > 0 && cpm.PcktLen > 0)
}

// CmpPtnFuncInstHost returns the name of the host to which the CmpPtnFuncInst given as argument is mapped.
func CmpPtnFuncInstHost(cpfi *CmpPtnFuncInst) string {
	cpfLabel := cpfi.funcLabel()
	cpfCmpPtn := cpfi.funcCmpPtn()

	return CmpPtnMapDict.Map[cpfCmpPtn].FuncMap[cpfLabel]
}

func isNOP(op string) bool {
	return (op == "noop" || op == "NOOP" || op == "NOP" || op=="no-op" || op=="nop")
}

// FuncExecTime returns the increase in execution time resulting from executing the
// CmpPtnFuncInst offered as argument, to the message also offered as argument.
// If the pcktlen of the message does not exactly match the pcktlen parameter of a func timing entry,
// an interpolation or extrapolation of existing entries is performed.
func FuncExecTime(cpfi *CmpPtnFuncInst, op string, msg *CmpPtnMsg) float64 {
	// get the parameters needed for the func execution time lookup
	if isNOP(op) {
		return 0.0
	}

	hostLabel := CmpPtnMapDict.Map[cpfi.funcCmpPtn()].FuncMap[cpfi.funcLabel()]

	cpumodel := netportal.EndptCPUModel(hostLabel)

	// if we don't have an entry for this function type, complain
	_, present := funcExecTimeTbl[op]
	if !present {
		panic(fmt.Errorf("no function timing look up for operation %s", op))
	}

	// if we don't have an entry for the named CPU for this function type, throw an error
	lenMap, here := funcExecTimeTbl[op][cpumodel]
	if !here || lenMap == nil {
		panic(fmt.Errorf("no function timing look for operation %s on cpu model %s", op, cpumodel))
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

	// see if this function is active, if not, bye
	if !cpfi.funcActive() {
		return nil
	}

	initiating := cpfi.isInitiating()

	var cpm *CmpPtnMsg
	if cpMsg != nil {
		cpm = cpMsg.(*CmpPtnMsg)
	}

	endptName := cpfi.host
	endpt := mrnes.EndptDevByName[endptName]

	// need to create an initial message?
	if initiating && cpm == nil {
		cpm = new(CmpPtnMsg)
		*cpm = *cpfi.InitMsg

		// make the source information reflect this function.
		// leave destination information off 
		cpm.CmpHdr.SrtCPID = cpfi.CPID
		cpm.CmpHdr.SrtLabel = cpfi.Label
		cpm.prevCPID = cpfi.CPID
		cpm.prevLabel = cpfi.Label

		// the 'nxt' fields prepared for the copy of 'nxt' back to 'prev'
		// when the message is updated
		cpm.nxtCPID  = cpfi.CPID
		cpm.nxtLabel = cpfi.Label

		// flag to start the timer on this execution thread
		cpm.Start = true

		cpm.ExecID = numExecThreads
		cpm.MsgType = "initiate"
		numExecThreads += 1

		if cpfi.funcTrace() {
			// work out the endpoint device and log the entry there
			traceMgr.AddTrace(evtMgr.CurrentTime(), cpm.ExecID, 0, endpt.DevID(), "enter", cpm.carriesPckt(), cpm.Rate)
			traceMgr.AddTrace(evtMgr.CurrentTime(), cpm.ExecID, 0, cpfi.ID, "enter", cpm.carriesPckt(), cpm.Rate)
		}
	} else if cpfi.funcTrace() {
		traceMgr.AddTrace(evtMgr.CurrentTime(), cpm.ExecID, 0, cpfi.ID, "enter", cpm.carriesPckt(), cpm.Rate)
	}

	// start the timer if required
	if cpm.Start {
		traceMgr.AddTrace(evtMgr.CurrentTime(), cpm.ExecID, 0, endpt.DevID(), "enter", cpm.carriesPckt(), cpm.Rate)
		cpi := CmpPtnInstByName[cpfi.funcCmpPtn()]
		traceName := cpi.name + ":" + cpfi.funcLabel()
		startRecExec(traceName, cpm.ExecID, cpfi.CPID, cpfi.funcLabel(), evtMgr.CurrentSeconds())
		cpm.Start = false
	}

	// get the method code associated with the message arrival from the named source
	es := edgeStruct{CPID: cpm.prevCPID, FuncLabel: cpm.prevLabel, MsgType: cpm.MsgType}
	methodCode, present := cpfi.inEdgeMethodCode[es]
	if !present {
		fmt.Printf("function %s receives unrecognized input message type %s, ignored\n",
			cpfi.funcLabel(), cpm.MsgType)
		return nil
	}

	// get the functions that start, and stop the function execution
	methods := cpfi.respMethods[methodCode]

	methods.Start(evtMgr, cpfi, methodCode, cpm)

	// if function not now active stop the collection of information about the execution thread
	if !cpfi.funcActive() {
		EndRecExec(cpm.ExecID, evtMgr.CurrentSeconds())
	}
	return nil
}

// EmptyInitFunc exists to detect when there is actually an initialization event handler
// (by having a 'emptyInitFunc' below be re-written to point to something else
func EmptyInitFunc(evtMgr *evtm.EventManager, cpFunc any, cpMsg any) any {
	return nil
}

var emptyInitFunc evtm.EventHandlerFunction = EmptyInitFunc

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
	if cpfi.funcTrace() {
		traceMgr.AddTrace(evtMgr.CurrentTime(), cpm.ExecID, 0, cpfi.ID, "exit", cpm.carriesPckt(), cpm.Rate)
	}

	// done if no-where to go
	if len(msgs) == 0 || len(cpm.nxtLabel) == 0 {
		// this is unexpected if len(cpm.Edge.DstLabel) == 0
		if len(cpm.nxtLabel) == 0 {
			print("unexpected ExitFunc call with zero-ed destination label")
		}
		return nil
	}

	// get a pointer to the comp pattern the function being exited belongs to
	cpi := CmpPtnInstByName[cpfi.funcCmpPtn()]

	// treat each msg individually
	for _, msg := range msgs {
		// the CmpPtnInst should have representation for the next function (which
		// will be the destination label of the edge embedded in the message).  Check, and
		// panic if not

		// allow for possibility that the next comp pattern is different
		xcpi := cpi
		if msg.prevCPID != msg.nxtCPID {
			xcpi = CmpPtnInstByID[msg.nxtCPID]
		}

		// notice that if the destination CP is different from the source, we
		// expect that the code which recreated msg put in the the edge the label
		// within the destination CP of the target
		nxtf, present := xcpi.funcs[msg.nxtLabel]
		if present {
			dstHost := CmpPtnMapDict.Map[xcpi.name].FuncMap[nxtf.Label]

			// Staying on the host means scheduling w/o delay the arrival at the next func
			if cpfi.host == dstHost {
				evtMgr.Schedule(nxtf, msg, EnterFunc, vrtime.SecondsToTime(0.0))
			} else {
				// to get to the dstHost we need to go through the network
				isPckt := msg.carriesPckt()
				netportal.EnterNetwork(evtMgr, cpfi.host, dstHost, msg.MsgLen, cpm.ExecID, isPckt, msg.Rate, msg,
					nxtf, ReEnter, xcpi, LostCmpPtnMsg)
			}
		} else {
			panic("missing dstLabel in CmpPtnInst description")
		}
	}

	return nil
}

func ReEnter(evtMgr *evtm.EventManager, cpFunc any, msg any) any {
	evtMgr.Schedule(cpFunc, msg, EnterFunc, vrtime.SecondsToTime(0.0))
	return nil
}

// LostCmpPtnMsg is scheduled from the mrnes side to report the loss of a comp pattern message
func LostCmpPtnMsg(evtMgr *evtm.EventManager, context any, msg any) any {
	cpMsg := msg.(*CmpPtnMsg)

	// cpMmsg.CP is the name of pattern
	EndCPID := cpMsg.CmpHdr.EndCPID
	execID := cpMsg.ExecID

	// look up a description of the comp pattern
	cpi := CmpPtnInstByID[EndCPID]

	cpi.activeCnt[execID] -= 1

	// if no other msgs active for this execID report loss
	if cpi.activeCnt[execID] == 0 {
		fmt.Printf("Comp Pattern %s lost message for execution id %d\n", cpi.name, execID)
		return nil
	}
	return nil
}

// numExecThreads is used to place a unique integer code on every newly created initiation message
var numExecThreads int = 1

// scheduleInitEvts goes through all CmpPtnInsts and for every self-initiating CmpPtnFuncInst
// on it schedules the initiation event handler
func schedInitEvts(evtMgr *evtm.EventManager) {
	// loop over all comp pattern instances
	for _, cpi := range CmpPtnInstByName {
		// loop over all of its funcs
		for _, cpFunc := range cpi.funcs {
			// see if this function calls for an initialization function and
			// schedule it if so
			if cpFunc.funcInitEvtHdlr() != nil {
				evtMgr.Schedule(cpFunc, nil, cpFunc.funcInitEvtHdlr(), vrtime.SecondsToTime(0.0))
			}
		}
	}
}
