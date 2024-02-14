package mrnesbits

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"strconv"
	"strings"
)

// cpg-rf.go holds code related to the computation pattern graph stateful functions

// functions called to implement a stateful functions funcExec are of type RespFunc
type RespFunc func(*evtm.EventManager, *cmpPtnStatefulFuncInst, *cmpPtnMsg) (float64, []*cmpPtnMsg)

// respTbl maps code names for response functions to the corresponding func
var respTbl map[string]RespFunc = make(map[string]RespFunc)

// IsRespFunc determines whether the input string has a corresponding response function
func IsRespFunc(name string) bool {
	_, present := respTbl[name]
	return present
}

func updateMsgEdge(msgptr *cmpPtnMsg, cpsif *cmpPtnStatefulFuncInst, sfr statefulFuncOut, label string) {
	msgptr.edge = createCmpPtnGraphEdge(cpsif.label, sfr.msgType, sfr.dstLabel, label)
}

func buildRespTbl() {
	if len(respTbl) == 0 {
		// default returns delay associated with function type on CPU,
		// and either response marked 'default' or all of them. Is execution thread entry point.
		respTbl["default"] = defaultFunc

		// cycle returns delay associated with function type on CPU, selects 'the next' host to get a response
		// and puts that selection in the cmpPtnMsg header. Is execution thread entry point.
		respTbl["cycle"] = cycleFunc
		respTbl["select"] = selectFunc
		respTbl["passThru"] = passThruFunc
		respTbl["encrypt-aes"] = passThruFunc
		respTbl["encrypt-rsa"] = passThruFunc
		respTbl["decrypt-aes"] = passThruFunc
		respTbl["decrypt-rsa"] = passThruFunc

		// generateFlag sets a certificate flag, periodically setting it to false. Is execution thread entry point
		respTbl["mark"] = generateFlag

		// validCert checks whether
		respTbl["testCert"] = validCert
	}
}

func defaultFunc(evtMgr *evtm.EventManager, cpsfi *cmpPtnStatefulFuncInst, msg *cmpPtnMsg) (float64, []*cmpPtnMsg) {
	ies := inEdge(msg)
	delay := respActionExecTime(cpsfi, ies, msg)

	msgs := []*cmpPtnMsg{}
	for _, funcOut := range cpsfi.resp[ies] {
		newMsg := new(cmpPtnMsg)
		*newMsg = *msg
		updateMsgEdge(newMsg, cpsfi, funcOut, "")
		msgs = append(msgs, newMsg)

		// but if there is a default we return just that
		if !(funcOut.choice == "default" || funcOut.choice == "Default") {
			msgs = []*cmpPtnMsg{newMsg}

			return delay, msgs
		}
	}

	return delay, msgs
}

//----- functions for the cryptoPerf comp pattern

// cycle chooses a destination host, cycling through a list of possibilities.
// It is called to respond to a self-initiated message from the source
// function in the StatefulTest CompPattern, and, depending on the state,
// generate a cmpPtnMsg in response, marked with a choosen destination
func cycleFunc(evtMgr *evtm.EventManager, cpsfi *cmpPtnStatefulFuncInst, msg *cmpPtnMsg) (float64, []*cmpPtnMsg) {
	ies := inEdge(msg)
	delay := respActionExecTime(cpsfi, ies, msg)

	// respList gets list of statefulFuncOut responses possible given input edge ies.
	// should be only one
	respList := cpsfi.resp[ies]

	// push a response out, 'the next' host
	_, present := cpsfi.state["hosts"]
	if !present {
		panic(fmt.Errorf("cycle function for %s needs state variable 'hosts'", cpsfi.label))
	}

	// recover the number of calls to this action
	callnumber := cpsfi.actions[ies].calls - 1

	// split the hosts string to make an array of names
	hosts := strings.Split(cpsfi.state["hosts"], ",")
	idx := callnumber % len(hosts)
	dstName := hosts[idx]

	msg.cmpHdr.endLabel = dstName
	updateMsgEdge(msg, cpsfi, respList[0], "")
	outputMsgs := []*cmpPtnMsg{msg}

	return delay, outputMsgs
}

// selectFunc picks out the destination header from the message, and looks for
// an output edge whose 'choice' field is the same, and pushes the message down that edge.
// This is more general than looking for a match with the dstLabel on the edge, because
// we can direct the flow in the direction of the destination w/o being adjacent
func selectFunc(evtMgr *evtm.EventManager, cpsfi *cmpPtnStatefulFuncInst, msg *cmpPtnMsg) (float64, []*cmpPtnMsg) {
	ies := inEdge(msg)
	delay := respActionExecTime(cpsfi, ies, msg)

	dstHost := msg.cmpHdr.endLabel

	// go through the responses looking for one where the choice string is the same as dstHost
	outputMsgs := []*cmpPtnMsg{}
	for _, resp := range cpsfi.resp[ies] {
		if resp.choice == dstHost {
			updateMsgEdge(msg, cpsfi, resp, "")
			outputMsgs = []*cmpPtnMsg{msg}

			break
		}
	}

	return delay, outputMsgs
}

// passThruFunc can be used when there is only one output associated with
// the input.  All that is done is to add a delay and pass the message through
func passThruFunc(evtMgr *evtm.EventManager, cpsfi *cmpPtnStatefulFuncInst, msg *cmpPtnMsg) (float64, []*cmpPtnMsg) {
	ies := inEdge(msg)
	delay := respActionExecTime(cpsfi, ies, msg)

	// there should be exactly one return possible
	resp := cpsfi.resp[ies][0]
	if len(resp.dstLabel) > 0 {
		updateMsgEdge(msg, cpsfi, resp, "")

		return delay, []*cmpPtnMsg{msg}
	}

	return delay, []*cmpPtnMsg{}
}

//------- functions associated with "StatefulTest" CompPattern

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

// generateFlag is called to respond to a self-initiated message from the source
// function in the StatefulTest CompPattern, and generates a cmpPtnMsg in response.
// Every 'failperiod' visits it sets a 'valid' bit in the message payload to be false,
// it is otherwise true
func generateFlag(evtMgr *evtm.EventManager, cpsfi *cmpPtnStatefulFuncInst, msg *cmpPtnMsg) (float64, []*cmpPtnMsg) {
	ies := inEdge(msg)
	delay := respActionExecTime(cpsfi, ies, msg)

	// respList gets list of statefulFuncOut responses possible given input edge ies.
	// should be only one
	respList := cpsfi.resp[ies]

	// get number of calls so far
	calls := cpsfi.actions[ies].calls

	// look up the perioicity of failing the cert
	fperiod, _ := strconv.Atoi(cpsfi.state["failperiod"])

	// create a cert payload.  Valid is true if not a multiple of fperiod
	newcert := createCert(cpsfi.label, calls%fperiod != 0)
	msg.payload = newcert

	updateMsgEdge(msg, cpsfi, respList[0], "")
	outputMsgs := []*cmpPtnMsg{msg}

	return delay, outputMsgs
}

// validCert determines the response of the stateful function, based on the
// triggering message (which is passed as an input) and state in the 'state' variable map.  The response is described
// by a slice of cmpPtnMsg.
func validCert(evtMgr *evtm.EventManager, cpsfi *cmpPtnStatefulFuncInst, msg *cmpPtnMsg) (float64, []*cmpPtnMsg) {
	ies := inEdge(msg)
	delay := respActionExecTime(cpsfi, ies, msg)

	// Initialize the slice that will hold the set of all cmpPtnMsg that are released in response.
	// This construct allows multiple messages to be offered in response.
	outputMsgs := []*cmpPtnMsg{}

	// respList gets list of statefulFuncOut responses possible given input edge ies
	respList := cpsfi.resp[ies]

	responseChoice := "consumer1" // default outEdge selection label
	if cpsfi.state["test"] == "true" {
		valid := (msg.payload).(*cert).valid // get the 'cert is valid' flag

		if !valid {
			// func testCert flag is true but cert is not valid, means choose the "false" choice label
			responseChoice = "consumer2"
		}
	}

	for _, resp := range respList {
		if responseChoice == resp.choice {
			newMsg := new(cmpPtnMsg)
			*newMsg = *msg
			updateMsgEdge(newMsg, cpsfi, resp, "")
			outputMsgs = append(outputMsgs, newMsg)

			break
		}
	}

	return delay, outputMsgs
}

func respActionExecTime(cpsfi *cmpPtnStatefulFuncInst, ies InEdge, msg *cmpPtnMsg) float64 {
	costType := cpsfi.actions[ies].costType
	return funcExecTime(cpsfi, costType, msg)
}
