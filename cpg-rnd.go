package mrnesbits

import (
	"fmt"
	"github.com/iti/evt/evtm"
)

// a rndFuncOut struct describes an outEdge for a response in a random function,
// and a cummulative probability value.  The sequence of rndFuncOut instances
// associated with an InEdge will increase in this value, enabling a simple
// implementation of a general probability distribution
type rndFuncOut struct {
	cummProb float64
	dstLabel string
	msgType  string
}

// The cmpPtnRndFuncInst struct represents an instantiated instance of a function constrained to the 'random' execution model.
type cmpPtnRndFuncInst struct {
	period  float64 // for a self-initiating func 'period' is non-zero, and gives the time between successive self-initiations
	label   string  // an identifier for this func, unique within the instance of CompPattern holding it.  May be reused in different CompPattern definitions.
	host    string  // identity of the host to which this func is mapped for execution
	fType   string  // 'function type', copied from the 'FuncType' attribute of the desc function upon which this instance is based
	ptnName string  // name of the instantiated CompPattern holding this function
	id      int     // integer identity which is unique among all objects in the MrNesbits model
	active  bool    // flag indicating whether function is actively processing messages

	// at run-time start-up this function is initialized with all its responses.   For the random execution model,
	// the set of potential output edges may be constrained, but not determined solely by the input edge as with the static model.
	// Here, for a given input edge, 'resp' holds a slice of possible responses, including a probability weight
	resp map[InEdge][]rndFuncOut

	// buffer the output messages created by executing the function body, indexed by the execId of the
	// initiating message
	msgResp map[int][]*cmpPtnMsg
}

// createRndFuncInst is a constructor that builds an instance of cmpPtnRndFunctInst from a Func description and
// a serialized representation of a RndParameters struct.
func createRndFuncInst(cpInstName string, fnc *Func, paramStr string) *cmpPtnRndFuncInst {
	rfi := new(cmpPtnRndFuncInst)
	rfi.id = nxtId()         // get an integer id that is unique across all objects in the simulation model
	rfi.fType = fnc.FuncType // remember the function type of the Func which is the base for this instance
	rfi.label = fnc.Label    // remember a label given to this function instance as part of building a CompPattern graph
	rfi.ptnName = cpInstName // remember the name of the instance of the comp pattern in which this func resides

	// look up the func-to-host assignment, established earlier in the initialization sequence
	rfi.host = cmpPtnFuncHost(rfi)

	// deserialize the encoded RndParameters struct.   Force use of YAML because we had to have YAML when serializing
	params, _ := DecodeRndParameters(paramStr)

	// we'll reset period if the static parameters initialization struct tells us to
	rfi.period = 0.0
	rfi.active = true

	rfi.resp = make(map[InEdge][]rndFuncOut)

	// build up the response distribution encoded in the RndParameters structure
	for _, res := range params.Response {
		// every InEdge gets at most one response.  That response is described by a list of OutEdges
		// weighted by a cummulative probability. Initialize that list
		rfi.resp[res.InEdge] = make([]rndFuncOut, 0)

		// cummProb is a cummulative probability value, created from the res.MsgSelect entries
		cummProb := float64(0.0)

		// a list of all possibly output edge responses are given in the MsgSelect list, each weighted by a probability mass value.
		// Iterate over the list, create instances of rndFuncOut structures, and embed these with the cummulative probability mass
		for outEdge, dstProb := range res.MsgSelect {
			rfi.resp[res.InEdge] = append(rfi.resp[res.InEdge], rndFuncOut{cummProb: cummProb + dstProb, dstLabel: outEdge.DstLabel, msgType: outEdge.MsgType})
			cummProb += dstProb
		}
		// make sure the last entry in the cummulative probability table is 1.0.  If
		// res.MsgSelect is properly constructed it will be
		rfi.resp[res.InEdge][len(rfi.resp[res.InEdge])-1].cummProb = 1.0
	}

	return rfi
}

// funcActive indicates whether function is actively processing messages
func (cprfi *cmpPtnRndFuncInst) funcActive() bool {
	return cprfi.active
}

// funcType gives the FuncType declares on the Fun upon which this instance is based
func (cprfi *cmpPtnRndFuncInst) funcType() string {
	return cprfi.fType
}

// funcLabel gives the unique-within-cmpPtnInst label identifier for the function
func (cprfi *cmpPtnRndFuncInst) funcLabel() string {
	return cprfi.label
}

// funcXtype returns the enumerated type 'static' to indicate that's this cmpPtnFunc's execution type
func (cprfi *cmpPtnRndFuncInst) funcXtype() funcExecType {
	return rnd
}

// funcId returns the unique-across-model integer identifier for this cmpPtnFunc
func (cprfi *cmpPtnRndFuncInst) funcId() int {
	return cprfi.id
}

// funcHost returns the name of the topological Host upon which this cmpPtnFunc executes
func (cprfi *cmpPtnRndFuncInst) funcHost() string {
	return cprfi.host
}

// funcCmpPtn gives the name of the cmpPtnInst the cmpPtnFunc is part of
func (cprfi *cmpPtnRndFuncInst) funcCmpPtn() string {
	return cprfi.ptnName
}

// func funcPeriod gives the delay (in seconds) between self-initiations, for cmpPtnFuncs that self-initiate.  Otherwise returns the default value, 0.0
func (cprfi *cmpPtnRndFuncInst) funcPeriod() float64 {
	return cprfi.period
}

// func funcDelay returns the delay (in seconds) associated with one execution of the cmpPtnFunc.
// It calls funcMsg to get this delay and the messages that are generated by the execution,
// saves the messages and returns the delay
func (cprfi *cmpPtnRndFuncInst) funcDelay(evtMgr *evtm.EventManager, msg *cmpPtnMsg) float64 {

	// perform whatever execution the instantiation supports, returning
	// a delay until the output messages are released, and the messages themselves.
	// the messages are buffered until the event of exiting the function is performed
	delay, resp := cprfi.funcExec(evtMgr, msg)

	// save the response messages, indexed by msg.execId
	cprfi.AddResponse(msg.execId, resp)

	return delay
}

// AddResponse stores the selected out message response from executing the function,
// to be released later.  Saving through cpfsi.resp map to account for concurrent overlapping executions
func (cprsi *cmpPtnRndFuncInst) AddResponse(execId int, resp []*cmpPtnMsg) {
	cprsi.msgResp[execId] = resp
}

// funcResp returns the saved list of function response messages associated
// the the response to the input msg, and removes it from the msgResp map
func (cprsi *cmpPtnRndFuncInst) funcResp(execId int) []*cmpPtnMsg {
	rtn, present := cprsi.msgResp[execId]
	if !present {
		panic(fmt.Errorf("unsuccessful resp recovery\n"))
	}
	delete(cprsi.msgResp, execId)

	return rtn
}

// funcExec determines the response of the random function, based on the
// the InEdge of the triggering message (which is passed as an input).  The response is a
// probabilistic sampling from a distribution of edges associated with that input edge
func (cprfi *cmpPtnRndFuncInst) funcExec(evtMgr *evtm.EventManager, msg *cmpPtnMsg) (float64, []*cmpPtnMsg) {
	// construct a representation for an InEdge we can use to access the list of potential OutEdge responses
	ies := inEdge(msg)
	fType := cprfi.fType
	delay := funcExecTime(cprfi, fType, msg)

	// we need access to an RNG, there is one associated with the host, so get its name
	cmpPtnName := cprfi.funcCmpPtn()
	cpi := cmpPtnInstByName[cmpPtnName]

	// set respList to the list of rndFuncOut responses possible given input edge ies
	respList, present := cprfi.resp[ies]
	if !present {
		panic(fmt.Errorf("function %s fails to file list of responses associated with inEdge %v\n", cprfi.label, ies))
	}

	// respIdx will be the index of the edge we probabilistically choose.
	var respIdx = 0

	// if there is only one response to this input the response we select is 0.
	// otherwise ...
	if len(respList) > 0 {
		// cpi.rngs is a random number generator
		u := cpi.rngs.RandU01()

		// standard method for sampling from a probability distribution, choosing a uniform
		// and seeing where it lies w.r.t. the cummulative probability values.
		for idx := 0; idx < len(respList); idx++ {
			if u < respList[idx].cummProb {
				respIdx = idx

				break
			}
		}
	}

	// make a copy of the selected OutEdge
	var output = respList[respIdx]

	// modify 'edge' attribute of msg
	msg.edge.srcLabel = cprfi.funcLabel()

	// include the selected outedge material
	msg.edge.msgType = output.msgType
	msg.edge.dstLabel = output.dstLabel

	// we return only one of these but the interface calls for a slice.
	return delay, []*cmpPtnMsg{msg}
}
