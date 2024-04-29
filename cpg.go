// package mrnesbits implements a discrete-event simulator of computations distributed
// across a network, according to the MrNesbits design (Multiresolution Network and Emulation Simulator for BITS)
package mrnesbits

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
	"github.com/iti/rngstream"
	"math"
)

// CompPattern functions, messages, and edges are described by structs in the desc package,
// and at runtime are read into MrNesbits.  Runtime data-structures (such as CmpPtnInst
// and CmpPtnMsg) are created from constructors that take desc structures as arguments.

// given the name (alt., id) of an instance of a CompPattern, get a pointer to its struct
var CmpPtnInstByName map[string]*CmpPtnInst = make(map[string]*CmpPtnInst)
var CmpPtnInstById map[int]*CmpPtnInst = make(map[int]*CmpPtnInst)

type execRecord struct {
	src   string  // func initiating execution trace
	start float64 // time the trace started
}

type execSummary struct {
	n         int     // number of executions
	completed int     // number of completions
	sum       float64 // sum of measured execution times of completed
	sum2      float64 // sum of squared execution times
}

// CmpPtnInst describes a particular instance of a CompPattern,
// built from information read in from CompPatternDesc struct and used at runtime
type CmpPtnInst struct {
	name      string // this instance's particular name
	cpType    string
	id        int                        // unique id
	funcs     map[string]*CmpPtnFuncInst // use func label to get to func in that pattern with that label
	msgs      map[string]CompPatternMsg  // MsgType indexes msgs
	Rngs      *rngstream.RngStream
	Graph     *CmpPtnGraph                      // graph describing structure of funcs and edges
	active    map[int]execRecord                // executions that are active now
	activeCnt map[int]int                       // number of instances of executions with common execId (>1 by branching)
	lostExec  map[int]evtm.EventHandlerFunction // call this handler when a packet for a given execId is lost
	finished  map[string]execSummary            // summary of completed executions
}

// createCmpPtnInst is a constructor.  Inputs given by two structs from desc package.
// cpd describes the CompPattern, and cpid describes this instance's initialization parameters.
func createCmpPtnInst(ptnInstName string, cpd CompPattern, cpid CPInitList) (*CmpPtnInst, error) {
	cpi := new(CmpPtnInst)

	// the instance gets name on the input list
	cpi.name = ptnInstName

	// get assuredly unique id
	cpi.id = nxtId()

	// initialize slice of the func instances that make up the cpmPtnInst
	cpi.funcs = make(map[string]*CmpPtnFuncInst)

	// initialize map that carries information about active execution threads
	cpi.active = make(map[int]execRecord)

	// initialize map that carries number of active execs with same execid
	cpi.activeCnt = make(map[int]int)

	// initialize map that carries event scheduler to call if packet on execId is lost
	cpi.lostExec = make(map[int]evtm.EventHandlerFunction)

	// initialize map that carries information about completed executions
	cpi.finished = make(map[string]execSummary)

	// make a representation of the CmpPtnFuncInst where funcs are nodes and edges are
	// labeled with message type
	var gerr error
	cpi.Graph, gerr = createCmpPtnGraph(&cpd)

	// enable access to this struct through its name, and through its id
	CmpPtnInstByName[cpi.name] = cpi
	CmpPtnInstById[cpi.id] = cpi

	// The cpd structure has a list of desc descriptions of functions.
	// Call a constructor for each, depending on the func's execution type.
	// Save the created runtime representation of the function in the CmpPtnInst's list of funcs
	for _, funcDesc := range cpd.Funcs {
		if !validFuncClass(funcDesc.Class) {
			panic(fmt.Errorf("Function class %s not recognized\n", funcDesc.Class))
		}

		df := createFuncInst(ptnInstName, &funcDesc, cpid.Params[funcDesc.Label], cpid.State[funcDesc.Label], cpid.UseYAML)
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

// startRecExec records the name of the initialiating func, and starting
// time of an execution trace
func (cpi *CmpPtnInst) startRecExec(execId int, funcName string, time float64) {
	activeRec := execRecord{src: funcName, start: time}
	cpi.active[execId] = activeRec
	cpi.activeCnt[execId] = 1
}

// EndRecExec computes the completed execution time of the execution identified,
// given the ending time, incorporates its statistics into the CmpPtnInst
// statistics summary
func (cpi *CmpPtnInst) EndRecExec(execId int, time float64, completed bool) float64 {
	rtn := time - cpi.active[execId].start
	src := cpi.active[execId].src
	delete(cpi.active, execId)
	rec, present := cpi.finished[src]
	if !present {
		rec = execSummary{n: 0, sum: float64(0.0), sum2: float64(0.0), completed: 0}
		cpi.finished[src] = rec
	}
	rec.n += 1
	if completed {
		rec.completed += 1
		rec.sum += rtn
		rec.sum2 += rtn * rtn
	}
	cpi.finished[src] = rec
	return rtn
}

func (cpi *CmpPtnInst) cleanUp() {
	for execId := range cpi.active {
		delete(cpi.active, execId)
	}
}

// ExecReport computes and reports 95% confidence intervals on the execution time
// of an exection trace initiated at a labeled source on the cpi
func (cpi *CmpPtnInst) ExecReport() {
	cpi.cleanUp()

	for src, rec := range cpi.finished {
		if rec.n == 0 {
			continue
		}

		if rec.n == 1 {
			fmt.Printf("%s initiating function %s measures %f seconds\n", cpi.name, src, rec.sum)
		} else {
			loss := 1.0 - float64(rec.completed)/float64(rec.n)

			N := float64(rec.completed)
			rtN := math.Sqrt(N)
			m := rec.sum / N
			sigma2 := (rec.sum2 - (rec.sum*rec.sum)/N) / (N - 1)
			if sigma2 < 0.0 {
				sigma2 = 0.0
			}
			sigma := math.Sqrt(sigma2)
			w := 1.96 * sigma / rtN
			fmt.Printf("%s initiating function %s mean is %f +/- %f seconds with 95 percent' confidence using %d samples, loss percentage %f\n",
				cpi.name, src, m, w, rec.n, loss)
		}
	}
}

// endPts carries information on a CmpPtnMsg about where the execution thread started, and holds a place for a destination
type EndPts struct {
	SrtLabel string
	EndLabel string
}

// A CmpPtnMsg struct describes a message going from one CompPattern function to another.
// It carries ancillary information about the message that is included for referencing.
type CmpPtnMsg struct {
	ExecId int    // initialize when with an initating comp pattern message.  Carried by every resulting message.
	SrcCP  string // name of comp pattern instance from which the message originates
	DstCP  string // name of comp pattern instance to which the message is directed

	// what essential role does 'Edge' convey?
	Edge CmpPtnGraphEdge

	// do we need/want CmpHdr?
	CmpHdr EndPts

	MsgType string  // describes function of message
	MsgLen  int     // number of bytes
	PcktLen int     // parameter impacting execution time
	Rate    float64 // when non-zero, a rate limiting attribute that might used, e.g., in modeling IO
	Payload any     // free for "something else" to carry along and be used in decision logic
}

// carriesPckt indicates whether the message conveys information about a packet or a flow
func (cpm *CmpPtnMsg) carriesPckt() bool {
	return (cpm.MsgLen > 0 && cpm.PcktLen > 0)
}

// CreateCmpPtnMsg is a constructor whose arguments are all the attributes needed to make a CmpPtnMsg.
// One is created and returnedl
func CreateCmpPtnMsg(edge CmpPtnGraphEdge, srt, end string, msgType string, msgLen int, pcktLen int, rate float64, SrcCP, DstCP string,
	payload any, execId int) *CmpPtnMsg {

	// record that the srt point is the srcLabel on the message, but at this construction no destination is chosen
	cmphdr := EndPts{SrtLabel: srt, EndLabel: end}

	return &CmpPtnMsg{SrcCP: SrcCP, DstCP: DstCP, Edge: edge, CmpHdr: cmphdr, MsgType: msgType,
		MsgLen: msgLen, PcktLen: pcktLen, Rate: rate, Payload: payload, ExecId: execId}
}

// InEdgeMsg returns the InEdge structure embedded in a CmpPtnMsg
func InEdgeMsg(cmp *CmpPtnMsg) InEdge {
	ies := new(InEdge)

	// if the src and dst comp patterns are the same, return the src from the edge
	if cmp.SrcCP == cmp.DstCP {
		ies.SrcLabel = cmp.Edge.SrcLabel
	} else {
		// put in code for 'external'
		ies.SrcLabel = "***"
	}

	ies.MsgType = cmp.MsgType
	return *ies
}

// CmpPtnFuncInstHost returns the name of the host to which the CmpPtnFuncInst given as argument is mapped.
func CmpPtnFuncInstHost(cpfi *CmpPtnFuncInst) string {
	cpfLabel := cpfi.funcLabel()
	cpfCmpPtn := cpfi.funcCmpPtn()

	return CmpPtnMapDict.Map[cpfCmpPtn].FuncMap[cpfLabel]
}

// funcExecTime returns the increase in execution time resulting from executing the
// CmpPtnFuncInst offered as argument, to the message also offered as argument.
// If the pcktlen of the message does not exactly match the pcktlen parameter of a func timing entry,
// an interpolation or extrapolation of existing entries is performed.
func FuncExecTime(cpfi *CmpPtnFuncInst, op string, hardware string, msg *CmpPtnMsg) float64 {
	// get the parameters needed for the func execution time lookup
	hostLabel := CmpPtnMapDict.Map[cpfi.funcCmpPtn()].FuncMap[cpfi.funcLabel()]

	// if we were not given the hardware platform look up the default on the host CPU
	if len(hardware) == 0 {
		hardware = netportal.EndptCPU(hostLabel)
	}

	// if we don't have an entry for this function type, complain
	_, present := funcExecTimeTbl[op]
	if !present {
		panic(fmt.Errorf("no function timing look up for operation %s\n", op))
	}

	// if we don't have an entry for the named CPU for this function type, throw an error
	lenMap, here := funcExecTimeTbl[op][hardware]
	if !here || lenMap == nil {
		panic(fmt.Errorf("no function timing look for operation %s on hardware %s\n", op, hardware))
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
	cpi := CmpPtnInstByName[cpfi.funcCmpPtn()]

	// see if this function is active, if not, bye
	if !cpfi.funcActive() {
		return nil
	}

	initiating := cpfi.isInitiating()

	var cpm *CmpPtnMsg
	if cpMsg != nil {
		cpm = cpMsg.(*CmpPtnMsg)
	}

	if initiating && cpm == nil {
		cpm = new(CmpPtnMsg)
		*cpm = *cpfi.InitMsg
		cpm.ExecId = numExecThreads
		cpm.MsgType = "initiate"
		numExecThreads += 1
		// EnterFunc starts the clock on the execution thread
		cpi.startRecExec(cpm.ExecId, cpfi.funcLabel(), evtMgr.CurrentSeconds())
	}

	// note entry to function
	traceMgr.AddTrace(evtMgr.CurrentTime(), cpm.ExecId, 0, cpfi.id, "enter", cpm.carriesPckt(), cpm.Rate)

	// separating the methodCode from the response allows
	// different inedges to map to the same response
	// fc.Codes initialized when cpInit.yaml read in, from the func response entry
	methodCode, present := cpfi.methodCode[cpm.MsgType]
	if !present {
		fmt.Printf("function %s receives unrecognized input message type %s, ignored\n",
			cpfi.funcLabel(), cpm.MsgType)
		return nil
	}

	methods := cpfi.respMethods[methodCode]

	// call the response function associated with receiving a message on ies
	accepted, scheduleExit, delay, outMsgs := methods.Start(evtMgr, cpfi, methodCode, cpm)

	// if not accepted, stop the collection of information about the execution thread
	if !accepted {
		cpi.EndRecExec(cpm.ExecId, evtMgr.CurrentSeconds(), false)
	}

	// calling funcEnter could inactivate the function altogether.
	// this is different from not accepting the message
	if !cpfi.funcActive() || !accepted {
		return nil
	}

	// check whether the action completion can be scheduled
	if scheduleExit {
		// save the response output messages, indexed by msg.ExecId
		cpfi.AddResponse(cpm.ExecId, outMsgs)

		// if schedule the end method. N.B. this may be the ExitFunc handler if
		// so configured, but need not necessarily be so.  Buried in the table, we don't see it here
		evtMgr.Schedule(cpFunc, cpm, methods.End, vrtime.SecondsToTime(delay))
	}

	return nil
}

/*
func interarrivalSample(cpi *CmpPtnInst, dist string, mean float64) float64 {
	switch dist {
	case "const":
		return mean
	case "exp":
		u := cpi.Rngs.RandU01()
		return -1.0 * mean * math.Log(u)
	}
	return mean
}
*/

// EmptyInitFunc exists to detect when there is actually an initialization event handler
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
	msgs := cpfi.funcResp(cpm.ExecId)

	// note exit from function
	traceMgr.AddTrace(evtMgr.CurrentTime(), cpm.ExecId, 0, cpfi.id, "exit", cpm.carriesPckt(), cpm.Rate)

	// see if we're done
	if len(msgs) == 0 || len(cpm.Edge.DstLabel) == 0 {
		cpi := CmpPtnInstByName[cpfi.funcCmpPtn()]

		cpi.EndRecExec(cpm.ExecId, evtMgr.CurrentSeconds(), true)
		// fmt.Printf("at time %f execId %d hit the end of the function chain at %s with delay %f\n",
		//	evtMgr.CurrentSeconds(), cpm.ExecId, cpfi.funcLabel(), delay)
		return nil
	}

	// get a pointer to the comp pattern the function being exited belongs to
	cpi := CmpPtnInstByName[cpfi.funcCmpPtn()]

	// treat each msg individually
	for _, msg := range msgs {
		// the CmpPtnInst should have representation for the next function (which
		// will be the destination label of the edge embedded in the message).  Check, and
		// panic if not

		// allow for possibility that the destination comp pattern is different
		xcpi := cpi
		if msg.SrcCP != msg.DstCP {
			xcpi = CmpPtnInstByName[msg.DstCP]
		}

		// notice that if the destination CP is different from the source, we
		// expect that the code which recreated msg put in the the edge the label
		// within the destination CP of the target
		nxtf, present := xcpi.funcs[msg.Edge.DstLabel]
		if present {
			dstHost := CmpPtnMapDict.Map[xcpi.name].FuncMap[nxtf.Label]

			// Staying on the host means scheduling w/o delay the arrival at the next func
			if cpfi.host == dstHost {
				evtMgr.Schedule(nxtf, msg, EnterFunc, vrtime.SecondsToTime(0.0))
			} else {
				// to get to the dstHost we need to go through the network
				isPckt := msg.carriesPckt()
				netportal.EnterNetwork(evtMgr, cpfi.host, dstHost, msg.MsgLen, cpm.ExecId, isPckt, msg.Rate, msg,
					nil, ReEnter, cpFunc, LostCmpPtnMsg)

			}
		} else {
			panic("missing dstLabel in CmpPtnInst description")
		}
	}

	return nil
}

// reEnter is scheduled from the mrnes side to report the delivery
func ReEnter(evtMgr *evtm.EventManager, context any, msg any) any {
	cpMsg := msg.(*CmpPtnMsg)

	// cpMmsg.CP is the name of pattern
	CP := cpMsg.DstCP

	// look up a description of the comp pattern
	cpi := CmpPtnInstByName[CP]

	// name of the target function is part of nm.msg
	label := cpMsg.Edge.DstLabel

	// need the function to push on
	cpFunc := cpi.funcs[label]

	// schedule arrival at the function, after last bit clears interface.
	// N.B. should add some processing-time overhead here.
	evtMgr.Schedule(cpFunc, msg, EnterFunc, vrtime.SecondsToTime(0.0))

	return nil
}

// lostCmpPtnMsg is scheduled from the mrnes side to report the loss of a comp pattern message
func LostCmpPtnMsg(evtMgr *evtm.EventManager, context any, msg any) any {
	cpMsg := msg.(*CmpPtnMsg)

	// cpMmsg.CP is the name of pattern
	CP := cpMsg.DstCP
	execId := cpMsg.ExecId

	// look up a description of the comp pattern
	cpi := CmpPtnInstByName[CP]

	cpi.activeCnt[execId] -= 1

	// if no other msgs active for this execId report loss
	if cpi.activeCnt[execId] == 0 {
		fmt.Printf("Comp Pattern %s lost message for execution id %d\n", CP, execId)
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
