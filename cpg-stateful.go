package mrnesbits

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"strconv"
)

// file cpg-stateful.go instantiates the cmpPtnFunc interface for the 'stateful' execution model

// The statefulFuncOut struct describes a possible output edge for a response, along with a topology-free label
// 'choice' that can be used in the response selection process.
type statefulFuncOut struct {
	dstLabel string
	msgType  string
	choice   string
}

// The cmpPtnStatefulFuncInst struct represents an instantiated instance of a function
//
//	constrained to the 'statefulerminisitc' execution model.
type cmpPtnStatefulFuncInst struct {
	period float64 // for a self-initiating func 'period' is non-zero, and gives the time between successive self-initiations
	label  string  // an identifier for this func, unique within the instance of CompPattern holding it.
	// May be reused in different CompPattern definitions.
	host    string            // identity of the host to which this func is mapped for execution
	ptnName string            // name of the instantiated CompPattern holding this function
	id      int               // integer identity which is unique among all objects in the MrNesbits model
	active  bool              // flag whether function is actively processing inputs
	state   map[string]string // holds string-coded state for string-code state variable names
	actions map[InEdge]actionStruct

	// at run-time start-up this function is initialized with all its responses.   For the stateful execution model,
	// the set of potential output edges may be constrained, but not statefulermined solely by the input edge as with the static model.
	// Here, for a given input edge, 'resp' holds a slice of possible responses.
	resp map[InEdge][]statefulFuncOut

	// buffer the output messages created by executing the function body, indexed by the execId of the
	// initiating message
	msgResp map[int][]*cmpPtnMsg
}

type actionStruct struct {
	costType, actionSelect, completeSelect string
	calls, limit           int
}

// createDestFuncInst is a constructor that builds an instance of cmpPtnStatefulFunctInst from a Func description and
// a serialized representation of a StaticParameters struct.
func createStatefulFuncInst(cpInstName string, fnc *Func, paramStr string, newState map[string]string, useYAML bool) *cmpPtnStatefulFuncInst {
	dfi := new(cmpPtnStatefulFuncInst)
	dfi.id = nxtId()                    // get an integer id that is unique across all objects in the simulation model
	dfi.label = fnc.Label               // remember a label given to this function instance as part of building a CompPattern graph
	dfi.state = make(map[string]string) // initialize map for state variables
	dfi.ptnName = cpInstName            // remember the name of the instance of the comp pattern in which this func resides
	dfi.period = 0.0
	dfi.active = true

	dfi.resp = make(map[InEdge][]statefulFuncOut)
	dfi.msgResp = make(map[int][]*cmpPtnMsg)
	dfi.actions = make(map[InEdge]actionStruct)

	// look up the func-to-host assignment, established earlier in the initialization sequence
	dfi.host = cmpPtnFuncHost(dfi)

	// deserialize the encoded StatefulParameters struct
	params, _ := DecodeStatefulParameters(paramStr, useYAML)

	for _, respAction := range params.Actions {
		dfi.actions[respAction.Prompt] = actionStruct{costType: respAction.Action.CostType, 
			actionSelect: respAction.Action.Select, completeSelect: respAction.Action.Complete,
			calls: 0, limit: respAction.Limit}
	}

	// look for a response that flags a non-default (empty) self initiation
	for _, res := range params.Response {
		// there is at most one empty input edge response
		if res.InEdge.SrcLabel == dfi.label {
			dfi.period = res.Period

			break
		}
	}

	// copy over the state map
	for key, value := range params.State {
		dfi.state[key] = value
	}

	// if there is new state information, use it.  Add entries not present before, overwrite entries that are present
	if len(newState) > 0 {
		for key, value := range newState {
			dfi.state[key] = value
		}
	}
	// edges are maps of message type to func node labels. Here we copy the input list, but
	// gather together responses that have a common InEdge  (N.B., InEdge is a structure more complex than an int or string, but can be used to index maps)
	for _, res := range params.Response {
		_, present := dfi.resp[res.InEdge]

		// first time with this InEdge initialize the slice of responses
		if !present {
			dfi.resp[res.InEdge] = make([]statefulFuncOut, 0)
		}

		// create a response from the input response record
		df := statefulFuncOut{dstLabel: res.OutEdge.DstLabel, msgType: res.OutEdge.MsgType, choice: res.Choice}
		dfi.resp[res.InEdge] = append(dfi.resp[res.InEdge], df)
	}
	return dfi
}

// AddResponse stores the selected out message response from executing the function,
// to be released later.  Saving through cpfsi.resp map to account for concurrent overlapping executions
func (cpfsi *cmpPtnStatefulFuncInst) AddResponse(execId int, resp []*cmpPtnMsg) {
	cpfsi.msgResp[execId] = resp
}

// funcResp returns the saved list of function response messages associated
// the the response to the input msg, and removes it from the msgResp map
func (cpfsi *cmpPtnStatefulFuncInst) funcResp(execId int) []*cmpPtnMsg {

	/*
	ies := inEdge(msg)
	excAction, present := cpsfi.actions[ies]
	if !present {
		panic(fmt.Errorf("cmpPtnInst lists no function record for %v\n", ies))
	}

	actionCode := excAction.actionSelect
	_, present = respTbl[actionCode]
	if !present {
		panic(fmt.Errorf("cmpPtnInst lists no function response for %s\n", actionCode))
	}
	*/


	rtn, present := cpfsi.msgResp[execId]
	if !present {
		panic(fmt.Errorf("unsuccessful resp recovery\n"))
	}
	delete(cpfsi.msgResp, execId)

	return rtn
}

// funcActive indicates whether the function is processing messages
func (cpsfi *cmpPtnStatefulFuncInst) funcActive() bool {
	return cpsfi.active
}

// funcType gives the FuncType declares on the Fun upon which this instance is based
func (cpsfi *cmpPtnStatefulFuncInst) funcType() string {
	return ""
}

// funcLabel gives the unique-within-cmpPtnInst label identifier for the function
func (cpsfi *cmpPtnStatefulFuncInst) funcLabel() string {
	return cpsfi.label
}

// funcXtype returns the enumerated type 'stateful' to indicate that's this cmpPtnFunc's execution type
func (cpsfi *cmpPtnStatefulFuncInst) funcXtype() funcExecType {
	return stateful
}

// funcId returns the unique-across-model integer identifier for this cmpPtnFunc
func (cpsfi *cmpPtnStatefulFuncInst) funcId() int {
	return cpsfi.id
}

// funcCmpPtn gives the name of the cmpPtnInst the cmpPtnFunc is part of
func (cpsfi *cmpPtnStatefulFuncInst) funcCmpPtn() string {
	return cpsfi.ptnName
}

// funcHost returns the name of the topological Host upon which this cmpPtnFunc executes
func (cpsfi *cmpPtnStatefulFuncInst) funcHost() string {
	return cpsfi.host
}

// func funcPeriod gives the delay (in seconds) between self-initiations, for cmpPtnFuncs that self-initiate.
// Otherwise returns the default value, 0.0
func (cpsfi *cmpPtnStatefulFuncInst) funcPeriod() float64 {
	period := cpsfi.period

	// look for period over-write
	periodStr, present := cpsfi.state["period"]
	if present {
		period, _ = strconv.ParseFloat(periodStr, 64)
	}

	// if state["visitLimit"] is not set we're done
	_, present = cpsfi.state["visitLimit"]
	if !present {
		return period
	}
	limit, _ := strconv.Atoi(cpsfi.state["visitLimit"])
	visits, _ := strconv.Atoi(cpsfi.state["visits"])
	if visits < limit {
		return period
	}
	return 0.0
}

// func funcDelay returns the delay (in seconds) associated with one execution of the cmpPtnFunc.
// It calls funcMsg to get this delay and the messages that are generated by the execution,
// saves the messages and returns the delay
func (cpsfi *cmpPtnStatefulFuncInst) funcDelay(evtMgr *evtm.EventManager, msg *cmpPtnMsg) float64 {

	// perform whatever execution the instantiation supports, returning
	// a delay until the output messages are released, and the messages themselves.
	// the messages are buffered until the event of exiting the function is performed
	delay, resp := cpsfi.funcExec(evtMgr, msg)

	// save the response messages, indexed by msg.execId
	cpsfi.AddResponse(msg.execId, resp)

	return delay
}

// respOK increments the number of visits from the (embedded)
// input edge, and return false if that exceeds a limit
func (cpsfi *cmpPtnStatefulFuncInst) respOK(msg *cmpPtnMsg, incr bool) bool {

	// build a struct that characterizes the InEdge
	ies := inEdge(msg)
	exec := cpsfi.actions[ies]

	if incr {
		exec.calls += 1
	}

	limit := exec.limit

	allowed := true
	if limit > 0 && exec.calls > limit {
		allowed = false

		// turn processing off here
		cpsfi.active = false
	}

	cpsfi.actions[ies] = exec
	return allowed
}

// funcExec uses the actionSelect attribute of the func instance to look up
// and call the function associated with the function's response to the input
func (cpsfi *cmpPtnStatefulFuncInst) funcExec(evtMgr *evtm.EventManager, msg *cmpPtnMsg) (float64, []*cmpPtnMsg) {
	ies := inEdge(msg)

	excAction, present := cpsfi.actions[ies]
	if !present {
		panic(fmt.Errorf("cmpPtnInst lists no function record for %v\n", ies))
	}

	actionCode := excAction.actionSelect
	_, present = respTbl[actionCode]
	if !present {
		panic(fmt.Errorf("cmpPtnInst lists no function response for %s\n", actionCode))
	}

	// is it OK to respond?
	if cpsfi.respOK(msg, true) {
		// yes
		delay, resp := respTbl[actionCode](evtMgr, cpsfi, msg)
		return delay, resp
	}

	// wrap it up, we're done
	return 0.0, []*cmpPtnMsg{}
}
