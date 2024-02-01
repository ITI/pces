package mrnesbits

import (
	"strconv"
	"strings"
)

// file cpg-stateful.go instantiates the cmpPtnFunc interface for the 'stateful' execution model

// The statefulFuncOut struct describes a possible output edge for a response, along with a topology-free label
// 'choice' that can be used in the response selection process.
type statefulFuncOut struct {
	dstLabel string
	msgType  string
	choice   string
}

type RespFunc func(*cmpPtnStatefulFuncInst, *cmpPtnMsg) []*cmpPtnMsg

var respTbl map[string]RespFunc = make(map[string]RespFunc)

func IsRespFunc(name string) bool {
	_, present := respTbl[name]

	return present
}

var RespFuncCodes = []string{"mark", "testFlag", "filter"}

func buildRespTbl() {
	if len(respTbl) == 0 {
		respTbl["mark"] = generateFlag
		respTbl["testFlag"] = validCert
		respTbl["filter"] = filter
	}
}

// The cmpPtnStatefulFuncInst struct represents an instantiated instance of a function
//
//	constrained to the 'statefulerminisitc' execution model.
type cmpPtnStatefulFuncInst struct {
	period float64 // for a self-initiating func 'period' is non-zero, and gives the time between successive self-initiations
	label  string  // an identifier for this func, unique within the instance of CompPattern holding it.
	// May be reused in different CompPattern definitions.
	host    string // identity of the host to which this func is mapped for execution
	fType   string // 'function type', copied from the 'FuncType' attribute of the desc function upon which this instance is based
	ptnName string // name of the instantiated CompPattern holding this function
	id      int    // integer identity which is unique among all objects in the MrNesbits model

	state      map[string]string // holds string-coded state for string-code state variable names
	funcSelect string            // initialization defines this with a code the selects response behaviour on message input

	// at run-time start-up this function is initialized with all its responses.   For the stateful execution model,
	// the set of potential output edges may be constrained, but not statefulermined solely by the input edge as with the static model.
	// Here, for a given input edge, 'resp' holds a slice of possible responses.
	resp map[InEdgeStruct][]statefulFuncOut
}

// createDestFuncInst is a constructor that builds an instance of cmpPtnStatefulFunctInst from a Func description and
// a serialized representation of a StaticParameters struct.
func createStatefulFuncInst(cpInstName string, fnc *Func, paramStr string, useYAML bool) *cmpPtnStatefulFuncInst {
	dfi := new(cmpPtnStatefulFuncInst)
	dfi.id = nxtId()                    // get an integer id that is unique across all objects in the simulation model
	dfi.fType = fnc.FuncType            // remember the function type of the Func which is the base for this instance
	dfi.label = fnc.Label               // remember a label given to this function instance as part of building a CompPattern graph
	dfi.state = make(map[string]string) // initialize map for state variables
	dfi.ptnName = cpInstName            // remember the name of the instance of the comp pattern in which this func resides
	dfi.period = 0.0
	dfi.resp = make(map[InEdgeStruct][]statefulFuncOut)
	// look up the func-to-host assignment, established earlier in the initialization sequence
	dfi.host = cmpPtnFuncHost(dfi)
	// deserialize the encoded StatefulParameters struct
	params, _ := DecodeStatefulParameters(paramStr, useYAML)

	// look for a response that flags a non-default (empty) self initiation
	for _, res := range params.Response {
		// there is at most one empty input edge response
		if res.InEdge.SrcLabel == dfi.label {
			dfi.period = res.Period

			break
		}
	}
	dfi.funcSelect = params.FuncSelect
	dfi.state = params.State

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

// A cert struct is a simple device for the 'valid cert' example
type cert struct {
	name  string // name for the cert to distinguish it from others
	valid bool   // whether valid or not
}

// createCert is a constructor that uses its input arguments to create a cert, which is then returned.
func createCert(name string, value bool) *cert {
	cs := new(cert)
	cs.name = name
	cs.valid = value

	return cs
}

// funcType gives the FuncType declares on the Fun upon which this instance is based
func (cpsfi *cmpPtnStatefulFuncInst) funcType() string {
	return cpsfi.fType
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
	return cpsfi.period
}

// func funcDelay returns the delay (in seconds) associated with one execution of the cmpPtnFunc
func (cpsfi *cmpPtnStatefulFuncInst) funcDelay(msg *cmpPtnMsg) float64 {
	return funcExecTime(cpsfi, msg)
}

// funcExec uses the funcSelect attribute of the func instance to look up
// and call the function associated with the function's response to the input
func (cpsfi *cmpPtnStatefulFuncInst) funcExec(msg *cmpPtnMsg) []*cmpPtnMsg {
	return respTbl[cpsfi.funcSelect](cpsfi, msg)
}

// response functions...associated with "StatefulTest" CompPattern

// generateFlag is called to respond to a self-initiated message from the source
// function in the StatefulTest CompPattern, and generates a cmpPtnMsg in response.
// Every 'failperiod' visits it sets a 'valid' bit in the message payload to be false,
// it is otherwise true
func generateFlag(cpsfi *cmpPtnStatefulFuncInst, msg *cmpPtnMsg) []*cmpPtnMsg {
	// build a struct that characterizes the InEdge
	srcLabel := msg.edge.srcLabel
	msgType := msg.edge.msgType
	ies := InEdgeStruct{SrcLabel: srcLabel, MsgType: msgType}

	// respList gets list of statefulFuncOut responses possible given input edge ies.
	// should be only one
	respList := cpsfi.resp[ies]

	// increment the number of times this function has been called
	_, present := cpsfi.state["visits"]
	if !present {
		cpsfi.state["visits"] = "0"
	}

	visits, _ := strconv.Atoi(cpsfi.state["visits"])
	visits += 1
	cpsfi.state["visits"]  = strconv.Itoa(visits)

	fperiod, _ := strconv.Atoi(cpsfi.state["failperiod"])

	// create a cert payload.  Valid is true if not a multiple of fperiod
	newcert := createCert(cpsfi.label, visits%fperiod != 0)
	msg.payload = newcert

	nxtMsgType := respList[0].msgType
	nxtDstLabel := respList[0].dstLabel
	msg.edge = cmpPtnGraphEdge{srcLabel: msg.edge.srcLabel, msgType: nxtMsgType, dstLabel: nxtDstLabel}
	outputMsgs := []*cmpPtnMsg{msg}

	return outputMsgs
}

// validCert determines the response of the stateful function, based on the
// triggering message (which is passed as an input) and state in the 'state' variable map.  The response is described
// by a slice of cmpPtnMsgs.
func validCert(cpsfi *cmpPtnStatefulFuncInst, msg *cmpPtnMsg) []*cmpPtnMsg {
	// build a struct that characterizes the InEdge
	srcLabel := msg.edge.srcLabel
	msgType := msg.edge.msgType
	ies := InEdgeStruct{SrcLabel: srcLabel, MsgType: msgType}

	// Initialize the slice that will hold the set of all cmpPtnMsgs that are released in response.
	// This construct allows multiple messages to be offered in response.
	outputMsgs := []*cmpPtnMsg{}

	// respList gets list of statefulFuncOut responses possible given input edge ies
	respList := cpsfi.resp[ies]

	responseChoice := "consumer1" // default outEdge selection label
	if cpsfi.state["testflag"] == "true" {
		valid := (msg.payload).(*cert).valid // get the 'cert is valid' flag

		if !valid {
			// func testCert flag is true but cert is not valid, means choose the "false" choice label
			responseChoice = "consumer2"
		}
	}

	
	for _, resp := range respList {
		if responseChoice == resp.choice {
			output := resp
			newMsg := msg
			newMsg.edge.srcLabel = cpsfi.funcLabel()
			newMsg.edge.msgType = output.msgType
			newMsg.edge.dstLabel = output.dstLabel
			outputMsgs = append(outputMsgs, newMsg)

			break
		}
	}

	return outputMsgs
}

// filter is called when a message comes into, and the decision is to send
// a response or forward it on.  The filtering is applied only when the message
// is a request, which we will test for the word "request" in the edge label
func filter(cpsfi *cmpPtnStatefulFuncInst, msg *cmpPtnMsg) []*cmpPtnMsg {
	srcLabel := msg.edge.srcLabel
	msgType := msg.edge.msgType
	ies := InEdgeStruct{SrcLabel: srcLabel, MsgType: msgType}

	// Initialize the slice that will hold the set of all cmpPtnMsgs that are released in response.
	// This construct allows multiple messages to be offered in response.
	outputMsgs := []*cmpPtnMsg{}

	// respList gets list of statefulFuncOut responses possible given input edge ies
	respList := cpsfi.resp[ies]
	var resp statefulFuncOut
	if strings.Contains(msg.edge.edgeLabel, "request") {
		visits, _ := strconv.Atoi(cpsfi.state["visits"])
		visits += 1
		cpsfi.state["visits"]  = strconv.Itoa(visits)
		threshold, _ := strconv.Atoi(cpsfi.state["threshold"])
		if visits%threshold != 0 {
			// find the response whose srcLabel and dstLabels are the same
			for _, resp = range respList {
				// is the response heading right back to where it came from?
				if srcLabel == resp.dstLabel {
					// make a copy of the message, say it is coming from here, going to the
					// label indicated in the resp
					newMsg := msg
					newMsg.edge.srcLabel = cpsfi.label
					newMsg.edge.msgType = resp.msgType
					newMsg.edge.dstLabel = resp.dstLabel
					outputMsgs = append(outputMsgs, newMsg)

					// we're done
					return outputMsgs
				}
			}
		} else {
			// find the resp where the srcLabel is different from the dstLabel
			for _, resp = range respList {
				if srcLabel != resp.dstLabel {
					// the key here is that resp got set to what we need
					break
				}
			}
		}
	} else {
		// if the msg edge is not a request it is a response, and there is one element in respList
		resp = respList[0]
	}
	// resp now has the response to use. Copy the message, change the edge, send it along
	newMsg := msg
	newMsg.edge.srcLabel = cpsfi.label
	newMsg.edge.msgType = resp.msgType
	newMsg.edge.dstLabel = resp.dstLabel
	outputMsgs = append(outputMsgs, newMsg)

	return outputMsgs
}
