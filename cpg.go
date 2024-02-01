// package mrnesbits implements a discrete-event simulator of computations distributed
// across a network, according to the MrNesbits design (Multiresolution Network and Emulation Simulator for BITS)
package mrnesbits

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
	"github.com/iti/rngstream"
)

// CompPattern functions, messages, and edges are described by structs in the desc package,
// and at runtime are read into MrNesbits.  Runtime data-structures (such as cmpPtnInst
// and cmpPtnMsg) are created from constructors that take desc structures as arguments.

// given the name (alt., id) of an instance of a CompPattern, get a pointer to its struct
var cmpPtnInstByName map[string]*cmpPtnInst = make(map[string]*cmpPtnInst)
var cmpPtnInstById map[int]*cmpPtnInst = make(map[int]*cmpPtnInst)

// cmpPtnInst describes a particular instance of a CompPattern,
// built from information read in from CompPatternDesc struct and used at runtime
type cmpPtnInst struct {
	name     string                    // this instance's particular name
	cpType   string                    // this instance's comp pattern type
	id       int                       // unique id
	funcs    map[string]cmpPtnFunc     // use func label to get to func in that pattern with that label
	msgs     map[string]CompPatternMsg // MsgType indexes msgs
	rngs     *rngstream.RngStream
	graph    *cmpPtnGraph         // graph describing structure of funcs and edges
	initMsgs map[string]cmpPtnMsg // indexed by label of self-initiating Funcs
}

// createCmpPtnInst is a constructor.  Inputs given by two structs from desc package.
// cpd describes the CompPattern, and cpid describes this instance's initialization parameters.
func createCmpPtnInst(ptnInstName string, cpd CompPattern, cpid CPInitList) (*cmpPtnInst, error) {
	cpi := new(cmpPtnInst)

	// the instance gets the CompPattern Name
	cpi.name = cpd.Name

	// the instance saves the CompPattern CPType
	cpi.cpType = cpd.CPType

	// get assuredly unique id
	cpi.id = nxtId()

	// initialize slice of the func instances that make up the cpmPtnInst
	cpi.funcs = make(map[string]cmpPtnFunc)

	// make a representation of the cmpPtnFunc where funcs are nodes and edges are
	// labeled with message type
	var gerr error
	cpi.graph, gerr = createCmpPtnGraph(&cpd)

	// initialize slice that holds copies of messages used by funcs that are self-initiating
	cpi.initMsgs = make(map[string]cmpPtnMsg)

	// enable access to this struct through its name, and through its id
	cmpPtnInstByName[cpi.name] = cpi
	cmpPtnInstById[cpi.id] = cpi

	// The cpd structure has a list of desc descriptions of functions.
	// Call a constructor for each, depending on the func's execution type.
	// Save the created runtime representation of the function in the cmpPtnInst's list of funcs
	for _, funcDesc := range cpd.Funcs {
		switch funcDesc.ExecType {
		case "static":
			sf := createStaticFuncInst(ptnInstName, &funcDesc, cpid.Params[funcDesc.Label], cpid.UseYAML)
			cpi.funcs[sf.label] = sf

		case "stateful":
			df := createStatefulFuncInst(ptnInstName, &funcDesc, cpid.Params[funcDesc.Label], cpid.UseYAML)
			cpi.funcs[df.label] = df

		case "random":
			rf := createRndFuncInst(ptnInstName, &funcDesc, cpid.Params[funcDesc.Label])
			cpi.funcs[rf.label] = rf
		}
	}

	// save copies of all the messages for this CompPattern found in the initialization struct's list of messages
	cpi.msgs = make(map[string]CompPatternMsg)
	for _, msg := range cpid.Msgs {
		cpi.msgs[msg.MsgType] = msg
	}

	// create and save an rng
	cpi.rngs = rngstream.New(cpi.name)

	return cpi, gerr
}

// A cmpPtnMsg struct describes a message going from one CompPattern function to another.
// It carries ancillary information about the message that is included for referencing.
type cmpPtnMsg struct {
	execId     int     // initialize when with an initating comp pattern message.  Carried by every resulting message.
	initTime   float64 // time when the chain of messages started
	cmpPtnName string  // name of comp pattern instance within which this message flows
	edge       cmpPtnGraphEdge
	msgLen     int     // number of bytes
	pcktLen    int     // parameter impacting execution time
	payload    any     // free for "something else" to carry along and be used in decision logic
	rate       float64 // when non-zero, a rate limiting attribute that might used, e.g., in modeling IO
}

// createCmpPtnMsg is a constructor whose arguments are all the attributes needed to make a cmpPtnMsg.
// One is created and returnedl
func createCmpPtnMsg(edge cmpPtnGraphEdge, msgLen int, pcktLen int, cmpPtnName string,
	payload any, execId int, initTime float64) *cmpPtnMsg {
	return &cmpPtnMsg{cmpPtnName: cmpPtnName, edge: edge, msgLen: msgLen, pcktLen: pcktLen, payload: payload, execId: execId, initTime: initTime}
}

// funcExecType identifies the nature of the mapping from input message to output message
type funcExecType int

// Constants involved in enumerated type
const (
	static funcExecType = iota
	stateful
	rnd
)

// To accommodate different semantics of different execution types, we define an interface
// to each of the different execution type descriptive structs.  The control of function execution and
// passage of messages from one func to another is done at the level of the interface representation of the function.
// So, for instance, to schedule another event to follow after the execution delay at one func we call
// interface method funcDelay(*cmpPtnMsg) to return the delay, computed at the function execution type level but
// returned for use by the scheduler.

// cmpPtnFunc defines an interface for different func execution types
type cmpPtnFunc interface {
	funcCmpPtn() string               // identity of particular comp pattern this labeled func is on
	funcType() string                 // copy of the FuncType attribute of the underlaying Func
	funcLabel() string                // copy of the Label attribute of the underlaying Func
	funcHost() string                 // name of host this function is bound to
	funcPeriod() float64              // if >0 frequency of self-initiation (in seconds)
	funcXtype() funcExecType          // "static", "stateful", "rnd", etc.
	funcId() int                      // unique id within simulation model
	funcDelay(*cmpPtnMsg) float64     // increase in simulation time resulting from function execution
	funcExec(*cmpPtnMsg) []*cmpPtnMsg // result of applying function to a comp pattern message
}

// cmpPtnFuncHost returns the name of the host to which the cmpPtnFunc given as argument is mapped.
func cmpPtnFuncHost(cpf cmpPtnFunc) string {
	cpfLabel := cpf.funcLabel()
	cpfCmpPtn := cpf.funcCmpPtn()

	return cmpPtnMapDict.Map[cpfCmpPtn].FuncMap[cpfLabel]
}

// funcExecTime returns the increase in execution time resulting from executing the
// cmpPtnFunc offered as argument, to the message also offered as argument.
// If the pcktlen of the message does not exactly match the pcktlen parameter of a func timing entry,
// an interpolation or extrapolation of existing entries is performed.
func funcExecTime(cpf cmpPtnFunc, msg *cmpPtnMsg) float64 {
	// get the parameters needed for the func execution time lookup
	fType := cpf.funcType() // type of function measured, pattern independent
	hostLabel := cmpPtnMapDict.Map[cpf.funcCmpPtn()].FuncMap[cpf.funcLabel()]
	cpuType := netportal.HostCPU(hostLabel)
	
	// if we don't have an entry for this function type, complain
	_, present := funcExecTimeTbl[fType]
	if !present {
		panic(fmt.Errorf("no function timing look up for function type %s\n", fType))
	}

	// if we don't have an entry for the named CPU for this function type, throw an error
	lenMap, here := funcExecTimeTbl[fType][cpuType]
	if !here || lenMap == nil {
		panic(fmt.Errorf("no function timing look for function type %s on CPU %s\n", fType, cpuType))
	}

	// lenMap is map[int]string associating an execution time for a packet with the stated length,
	// so long as we have that length
	timing, present2 := lenMap[msg.pcktLen]
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

		return float64(msg.pcktLen) * timePerUnit
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

	return float64(msg.pcktLen)*m + b
}

// enterFunc is an event-handling routine, scheduled by an evtm.EventManager to execute and simulate the results of
// a message arrival to cmpPtnInst function.  The particular cmpPtnInst and particular message
// are given as arguments to the function.  A type-unspecified return is provided.
func enterFunc(evtMgr *evtm.EventManager, cpFunc any, cpMsg any) any {
	// extract the cmpPtnFunc and cmpPtnMsg involved in this event
	cpf := cpFunc.(cmpPtnFunc)
	cpm := cpMsg.(*cmpPtnMsg)

	// look up (or compute) the delay for the execution of the function, driven by the input message
	delay := cpf.funcDelay(cpm)

	// schedule the exit to occur after the computed delay
	evtMgr.Schedule(cpFunc, cpMsg, exitFunc, vrtime.SecondsToTime(delay))

	// if the message is an initiation, schedule the next one.  Detect
	// self-initiation when the edge on which the message arrived states
	// that the source and destination function labels are identical.
	if cpm.edge.srcLabel == cpf.funcLabel() {
		// In order to create a new message we must first get a pointer to the cmpPtnInst.
		// The cmpPtnFunc interface provides access to the name of the cmpPtnInst.
		cpi := cmpPtnInstByName[cpf.funcCmpPtn()]

		// The new message is a copy of the init message for this func, obtained from the func's initialization record.
		newInitMsg := new(cmpPtnMsg)
		*newInitMsg = cpi.initMsgs[cpf.funcLabel()]

		// write in the initTime
		newInitMsg.initTime = evtMgr.CurrentSeconds()

		// new message chain needs a unique execId code
		newInitMsg.execId = numExecThreads
		numExecThreads += 1

		// look up the time delay between successive self-initiations, and schedule the next one
		delay = cpf.funcPeriod()
		evtMgr.Schedule(cpFunc, newInitMsg, enterFunc, vrtime.SecondsToTime(delay))
	}

	return nil
}

// exitFunc is an event handling routine that implements the scheduling of messages which result from the completed
// (simulated) execution of a cmpPtnFunc.   The cmpPtnFunc and the message that triggered the execution
// are given as arguments to exitFunc.   This routine calls the cmpPtnFunc's function that computes
// the effect of doing the simulation, a routine which (by the cmpPtnFunc interface definition) returns a slice
// of cmpPtnMsgs which are then pushed further along the CompPattern chain.
func exitFunc(evtMgr *evtm.EventManager, cpFunc any, cpMsg any) any {
	// extract the cmpPtnFunc and cmpPtnMsg involved in this event
	cpf := cpFunc.(cmpPtnFunc)
	cpm := cpMsg.(*cmpPtnMsg)

	// get the response(s), if any.  Note that result is a slice of cmpPtnMsgs.
	msgs := cpf.funcExec(cpm)

	// treat each msg individually
	for _, msg := range msgs {
		// If the edge assocated with the msg has an empty dstLabel it means that
		// a branch is taken that ends the chain
		if msg.edge.dstLabel == "" {
			fmt.Printf("at time %f hit the end of the function chain at %s\n", evtMgr.CurrentSeconds(), cpf.funcLabel())

			continue
		}

		// get a pointer to the cmpPtnInst involved in this execution.
		cpi := cmpPtnInstByName[cpf.funcCmpPtn()]

		// the cmpPtnInst should have representation for the destination.  Check, and
		// panic if not
		nxtf, present := cpi.funcs[cpm.edge.dstLabel]
		if present {
			// get the dstLabel's host name
			hostName := cpf.funcHost()
			dstHost := cmpPtnMapDict.Map[cpf.funcCmpPtn()].FuncMap[nxtf.funcLabel()]

			// Staying on the host means scheduling w/o delay the arrival at the next func
			if hostName == dstHost {
				evtMgr.Schedule(nxtf, msg, enterFunc, vrtime.SecondsToTime(0.0))
			} else {
				// to get to the dstHost we need to go through the network
				netportal.EnterNetwork(evtMgr, hostName, dstHost, msg.msgLen, 0.0, msg, nil, reEnter)
			}
		} else {
			panic("missing dstLabel in cmpPtnInst description")
		}
	}

	return nil
}

// reEnter is scheduled from the mrnes side to report the delivery
func reEnter(evtMgr *evtm.EventManager, context any, msg any) any {
	cpMsg := msg.(*cmpPtnMsg)

	// cpMmsg.cmpPtnName is the name of pattern
	cmpPtnName := cpMsg.cmpPtnName

	// look up a description of the comp pattern
	cpi := cmpPtnInstByName[cmpPtnName]

	// name of the target function is part of nm.msg
	label := cpMsg.edge.dstLabel

	// need the function to push on
	cpFunc := cpi.funcs[label]

	// schedule arrival at the function, after last bit clears interface.
	// N.B. should add some processing-time overhead here.
	evtMgr.Schedule(cpFunc, msg, enterFunc, vrtime.SecondsToTime(0.0))

	return nil
}

// numExecThreads is used to place a unique integer code on every newly created initiation message
var numExecThreads int = 1

// scheduleInitEvts goes through all cmpPtnInsts and for every self-initiating cmpPtnFunc
// on it schedules the first arrival, first waiting the associated inter-initiation time.
func schedInitEvts(evtMgr *evtm.EventManager) {
	// loop over all comp pattern instances
	for _, cpi := range cmpPtnInstByName {
		// loop over all of its funcs
		for label, cpFunc := range cpi.funcs {
			// look up the inter-initiation interval, non-zero means self-initiation
			funcp := cpFunc.funcPeriod()
			if funcp > 0.0 {
				// will need to compare this func's label against ones in edge lists
				funcLabel := cpFunc.funcLabel()

				// search the func's inEdges for one where the srcLabel is the same as funcLabel
				edges := cpi.graph.nodes[label].inEdges

				for _, edgePtr := range edges {
					edge := *edgePtr

					if edge.srcLabel == funcLabel {
						// what is the message type for the self initiating message?
						edgeMsgType := edge.msgType

						// find pcktLen and msgLen values associated  with this message type on this comp pattern,
						// search the comp pattern's message for one with that same msgType
						var cpMsg *cmpPtnMsg = nil
						for _, msg := range cpi.msgs {
							if msg.MsgType == edgeMsgType {

								// found it, so create a message structure with empty payload
								// and trace (for now), and unique execution thread code
								cpMsg = createCmpPtnMsg(edge, msg.MsgLen, msg.PcktLen, cpi.name,
									nil, numExecThreads, funcp)
								numExecThreads += 1
								cpi.initMsgs[funcLabel] = *cpMsg
							}
						}
						// should have found this, if not, complain
						if cpMsg == nil {
							panic("Cannot find initiation edge")
						}

						// schedule the first initiation funcPeriod time in the future
						evtMgr.Schedule(cpFunc, cpMsg, enterFunc, vrtime.SecondsToTime(funcp))

						// only one of these per cmpPtnFunc, so we're done
						break
					}
				}
			}
		}
	}
}
