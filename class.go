package mrnesbits

import (
	"encoding/json"
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
	"gopkg.in/yaml.v3"
	"math"
	"strings"
)

// A FuncClass represents the methods used to simulate the effect
// of executing a function.  Different types of input generate different
// types of responses, so we use a map whose key selects the start, end pair of methods
type FuncClass interface {
	Class() string
	InitState(*CmpPtnFuncInst, string, bool)
}

// functions called to implement function's funcExec are of type RespFunc

type StartMethod func(*evtm.EventManager, *CmpPtnFuncInst, string,
	*CmpPtnMsg) (bool, bool, float64, []*CmpPtnMsg)

// RespMethod associates two RespFunc that implement a function's response,
// one when it starts, the other when it ends
type RespMethod struct {
	Start StartMethod
	End   evtm.EventHandlerFunction
}

// RegisterFuncClass is called to tell the system that a particular
// function class exists, and gives a point to its description.
// The idea is to register only those function classes actually used, or at least,
// provide clear separation between classes.  Return of bool allows call to RegisterFuncClass
// as part of a variable assignment outside of a function body
func RegisterFuncClass(fc FuncClass) bool {
	className := fc.Class()
	_, present := FuncClasses[className]
	if present {
		panic(fmt.Errorf("attempt to register function class %s after that name already registered", fc.Class()))
	}
	FuncClasses[className] = fc
	return true
}

// FuncClasses is a map that takes us from the name of a FuncClass
var FuncClasses map[string]FuncClass = make(map[string]FuncClass)

func UpdateMsg(msg *CmpPtnMsg, srcLabel, msgType, dstLabel, edgeLabel string) {
	msg.MsgType = msgType
	msg.Edge = CreateCmpPtnGraphEdge(srcLabel, msgType, dstLabel, edgeLabel)
}

func validFuncClass(class string) bool {
	_, present := FuncClasses[class]
	return present
}

//-------- methods and state for function class CryptoSrvr

// common class functions
// create the table entry needed to call the methods for this class

var csVar *cryptoSrvr = ClassCreateCryptoSrvr()
var registered bool = RegisterFuncClass(csVar)

func ClassCreateCryptoSrvr() *cryptoSrvr {
	cs := new(cryptoSrvr)
	cs.ClassName = "CryptoSrvr"
	cs.queue = []*queuedOp{}
	return cs
}

// cryptoSrvr carries the state information for an instance of this class
type cryptoSrvr struct {
	ClassName string
	Capacity  int    // maximum number of requests that can be queued up
	Algorithm string // code for crypto algorithm, in {"RSA","AES"} at present.  More to come maybe
	Hardware  string // code for the hardware used to do the encryption, overrides the host CPU if present
	// uncapitalized field will not be serialized/deserialized
	queue []*queuedOp // queue of crypto operations to perform
}

// ClassName returns the class name
func (cs *cryptoSrvr) Class() string {
	return cs.ClassName
}

// Populate initializes the CryptoSrvr struct when the model is being created initially
func (cs *cryptoSrvr) Populate(capacity int, algorithm, hardware string) {
	if !(capacity > 0) {
		panic(fmt.Errorf("CryptoSrvr capacity must be positive"))
	}

	found := false
	lowerAlg := strings.ToLower(algorithm)

	for _, alg := range Algorithms {
		if lowerAlg == strings.ToLower(alg) {
			found = true
			break
		}
	}
	if !found {
		panic(fmt.Errorf("CryptoSrvr algorithm must be in %v", Algorithms))
	}

	cs.Capacity = capacity
	cs.Algorithm = algorithm
	cs.Hardware = hardware
}

// InitState takes a string which encodes the intial representation of the function
// state and turns it into the class-dependent struct
func (cs *cryptoSrvr) InitState(cpfi *CmpPtnFuncInst, stateStr string, useYAML bool) {
	csVarAny, err := cs.Deserialize(stateStr, useYAML)
	if err != nil {
		panic(err)
	}
	csVar := csVarAny.(*cryptoSrvr)
	csVar.queue = []*queuedOp{}
	cpfi.State = csVar

	cpfi.AddStartMethod("encrypt-op", cryptoSrvrEncryptStart)
	cpfi.AddStartMethod("decrypt-op", cryptoSrvrDecryptStart)
}

// Support functions
// Serialize transforms the cryptoSrvr into string form
func (cs *cryptoSrvr) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*cs)
	} else {
		bytes, merr = json.Marshal(*cs)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a cryptoSrvr structure from its serialized representation
func (cs *cryptoSrvr) Deserialize(input string, useYAML bool) (any, error) {
	var err error
	// turn the string into a slice of bytes
	inputBytes := []byte(input)
	example := cryptoSrvr{Capacity: 0, Algorithm: "", Hardware: "", queue: []*queuedOp{}}

	// Select whether we read in json or yaml
	if useYAML {
		err = yaml.Unmarshal(inputBytes, &example)
	} else {
		err = json.Unmarshal(inputBytes, &example)
	}

	if err != nil {
		return nil, err
	}
	return &example, nil
}

// cryptoSrvrEncryptStart is called on entry to a function of the cryptoSrvr class.
func cryptoSrvrEncryptStart(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string,
	msg *CmpPtnMsg) (bool, bool, float64, []*CmpPtnMsg) {

	state := cpfi.State.(*cryptoSrvr)

	// if the server is at capacity already, return a flag saying the request was dropped
	if len(state.queue) == state.Capacity {
		fmt.Println("cryptoSrvr drops encrypt request")
		return false, false, 0.0, []*CmpPtnMsg{msg}
	}

	qOp := createQueuedOp("encrypt", msg)
	state.queue = append(state.queue, qOp)

	// if the load is now 1 we schedule the "completeService" event.  The time
	// depends on the operation, the algorithm, and the hardware

	if len(state.queue) == 1 {
		// create an op code that uses the "algorithm" string and adds "-encrypt"
		algOp := "encrypt-" + strings.ToLower(state.Algorithm)
		delay := FuncExecTime(cpfi, algOp, state.Hardware, msg)

		// schedule the service completion event
		evtMgr.Schedule(cpfi, nil, cryptoSrvrCompleteService, vrtime.SecondsToTime(delay))
	}

	// return tells control that the message was processed, but not to schedule the function exit.
	return true, false, 0.0, []*CmpPtnMsg{}
}

// cryptoSrvrEncryptStart is called on entry to a function of the cryptoSrvr class.
func cryptoSrvrDecryptStart(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string,
	msg *CmpPtnMsg) (bool, bool, float64, []*CmpPtnMsg) {

	state := cpfi.State.(*cryptoSrvr)

	// if the server is at capacity already, return a flag saying the request was dropped
	if len(state.queue) == state.Capacity {
		fmt.Println("cryptoSrvr drops decrypt request")
		return false, false, 0.0, []*CmpPtnMsg{msg}
	}

	qOp := createQueuedOp("decrypt", msg)
	state.queue = append(state.queue, qOp)

	// if the load is now 1 we schedule the "completeService" event.  The time
	// depends on the operation, the algorithm, and the hardware

	if len(state.queue) == 1 {
		// create an op code that uses the "algorithm" string and adds "-encrypt"
		algOp := "decrypt-" + strings.ToLower(state.Algorithm)
		delay := FuncExecTime(cpfi, algOp, state.Hardware, msg)

		// schedule the service completion event
		evtMgr.Schedule(cpfi, nil, cryptoSrvrCompleteService, vrtime.SecondsToTime(delay))
	}

	// return tells control that the message was processed, but not to schedule the function exit.
	return true, false, 0.0, []*CmpPtnMsg{}
}

// cryptoSrvrCompleteService handles the completion of service
func cryptoSrvrCompleteService(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	state := cpfi.State.(*cryptoSrvr)

	qOp := state.queue[0]
	// pop the queuedOp at the head of the queue
	state.queue = state.queue[1:]

	// if the queue has more to do, schedule the next completion
	if len(state.queue) > 0 {
		op := state.queue[0].cryptoOp
		algOp := op + "-" + state.Algorithm
		delay := FuncExecTime(cpfi, algOp, state.Hardware, state.queue[0].msg)

		// schedule the service completion event
		evtMgr.Schedule(cpfi, nil, cryptoSrvrCompleteService, vrtime.SecondsToTime(delay))
	}

	// complete operation on msg that just completed service
	msg := qOp.msg
	srcLabel := msg.Edge.SrcLabel

	// place the popped message where exitFunc will find it
	// change the edge on the message to reflect its next stop.
	// We get the destination label from the comp pattern instance graph, and the edge label by searching
	// the OutEdges for the function

	cpi := CmpPtnInstByName[cpfi.PtnName]

	// exitFunc will be looking at the msg edge label, expecting it to have been
	// updated for the next step
	//
	for _, outEdge := range cpi.Graph.Nodes[cpfi.Label].OutEdges {
		if outEdge.DstLabel != srcLabel {
			UpdateMsg(msg, cpfi.Label, outEdge.MsgType, outEdge.DstLabel, "")
			break
		}
	}

	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})

	// schedule the exitFunc handler
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

// queuedOp associates the type of crypto operation that is queued up to be
// performed with the message on which it is to be performed
type queuedOp struct {
	cryptoOp string
	msg      *CmpPtnMsg
}

// createQueueOp is a constructor, given all the arguments of the struct
func createQueuedOp(op string, msg *CmpPtnMsg) *queuedOp {
	return &queuedOp{cryptoOp: op, msg: msg}
}

//-------- methods and state for function class GenPckt

var gpVar *genPckt = ClassCreateGenPckt()
var genPcktLoaded bool = RegisterFuncClass(gpVar)

type genPckt struct {
	ClassName               string
	InterarrivalDist        string
	InterarrivalMean        float64
	InitMsgType             string
	InitMsgLen, InitPcktLen int
	InitiationLimit         int
	Calls                   int
}

func (gt *genPckt) Populate(mean float64, limit int, interarrival, msgType string, msgLen, pcktLen, calls int) {
	gt.Calls = calls
	gt.InitMsgLen = msgLen
	gt.InitPcktLen = pcktLen
	gt.InitMsgType = msgType
	gt.InterarrivalDist = interarrival
	gt.InterarrivalMean = mean
	gt.InitiationLimit = limit
}

func ClassCreateGenPckt() *genPckt {
	gt := new(genPckt)
	gt.ClassName = "GenPckt"
	gt.InitMsgType = ""
	gt.InitMsgLen = 0
	gt.InitPcktLen = 0
	gt.InterarrivalDist = "const"
	gt.InterarrivalMean = 0.0
	gt.InitiationLimit = 0
	return gt
}

func (gt *genPckt) Class() string {
	return gt.ClassName
}

func (gt *genPckt) InitState(cpfi *CmpPtnFuncInst, stateStr string, useYAML bool) {
	gtVarAny, err := gt.Deserialize(stateStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("genPckt.InitState for %s sees deserialization error", gt.ClassName))
	}

	gtv := gtVarAny.(*genPckt)
	cpfi.State = gtv
	cpfi.InitFunc = genPcktInitSchedule
	cpfi.InitMsgParams(gtv.InitMsgType, gtv.InitMsgLen, gtv.InitPcktLen, 0.0)
	cpfi.InterarrivalDist = gtv.InterarrivalDist
	cpfi.InterarrivalMean = gtv.InterarrivalMean
	cpfi.AddStartMethod("initiate-op", genPcktSelectStart)
	cpfi.AddStartMethod("return-op", genPcktReturnStart)
}

// InitSchedule schedules the next initiation at the comp pattern function instance pointed to
// by the context, and samples the next initiation time
func genPcktInitSchedule(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	gt := cpfi.State.(*genPckt)
	cpi := CmpPtnInstByName[cpfi.PtnName]

	gps := cpfi.State.(*genPckt)

	if gps.InitiationLimit > 0 && gps.InitiationLimit <= gps.Calls {
		return nil
	}

	// N.B. there should be a probability distribution package somewhere
	interarrival := gpinterarrivalSample(cpi, gt.InterarrivalDist, gt.InterarrivalMean)

	// schedule an arrival to the function at the comp pattern function
	evtMgr.Schedule(cpfi, nil, EnterFunc, vrtime.SecondsToTime(interarrival))

	// schedule the next self-initiation
	evtMgr.Schedule(cpfi, nil, genPcktInitSchedule, vrtime.SecondsToTime(interarrival))
	return nil
}

// serialize transforms the genPckt into string form for
// inclusion through a file
func (gt *genPckt) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*gt)
	} else {
		bytes, merr = json.Marshal(*gt)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a serialized representation of a genPckt structure
func (gt *genPckt) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)
	example := genPckt{Calls: 0}

	// Select whether we read in json or yaml
	if useYAML {
		err = yaml.Unmarshal(fsb, &example)
	} else {
		err = json.Unmarshal(fsb, &example)
	}

	if err != nil {
		return nil, err
	}
	return &example, nil
}

// genPcktSelectStart chooses a destination host from a list.  The selection
// is made as a function of a selection code.
func genPcktSelectStart(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string,
	msg *CmpPtnMsg) (bool, bool, float64, []*CmpPtnMsg) {

	state := cpfi.State.(*genPckt)
	state.Calls += 1

	ies := InEdge{SrcLabel: cpfi.Label, MsgType: "initiate"}
	respList := cpfi.Resp[ies]

	delay := FuncExecTime(cpfi, "generate-op", "", msg)
	msgType := respList[0].MsgType
	dstLabel := respList[0].DstLabel
	UpdateMsg(msg, cpfi.Label, msgType, dstLabel, "")

	outputMsgs := []*CmpPtnMsg{msg}

	return true, true, delay, outputMsgs
}

func gpinterarrivalSample(cpi *CmpPtnInst, dist string, mean float64) float64 {
	switch dist {
	case "const":
		return mean
	case "exp":
		u := cpi.Rngs.RandU01()
		return -1.0 * mean * math.Log(u)
	}
	return mean
}

// genPcktReturnStart handles the return message from one launched earlier.
// Prints a report
func genPcktReturnStart(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string,
	msg *CmpPtnMsg) (bool, bool, float64, []*CmpPtnMsg) {

	delay := FuncExecTime(cpfi, methodCode, "", msg)
	ies := InEdgeMsg(msg)
	respList := cpfi.Resp[ies]

	dstLabel := respList[0].DstLabel
	msgType := respList[0].MsgType

	UpdateMsg(msg, cpfi.Label, msgType, dstLabel, "")
	outputMsgs := []*CmpPtnMsg{msg}

	return true, true, delay, outputMsgs
}

var ppVar *pcktProcess = ClassCreatePcktProcess()
var ppLoaded = RegisterFuncClass(ppVar)

type pcktProcess struct {
	ClassName string
	Algorithm string // code for crypto algorithm, in {"RSA","AES"} at present.  More to come maybe
}

//------------- methods and state for function class PcktProcess

func ClassCreatePcktProcess() *pcktProcess {
	pp := new(pcktProcess)
	pp.ClassName = "PcktProcess"
	pp.Algorithm = "aes"
	return pp
}

func (pp *pcktProcess) Class() string {
	return pp.ClassName
}

func (pp *pcktProcess) InitState(cpfi *CmpPtnFuncInst, stateStr string, useYAML bool) {
	ppVarAny, err := pp.Deserialize(stateStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("pcktProcess.InitState for %s sees deserialization error", pp.ClassName))
	}
	ppVar := ppVarAny.(*pcktProcess)
	cpfi.State = ppVar
	cpfi.AddStartMethod("process-op", pcktProcessStart)
}

// Populate initializes the pcktProcess struct when the model is being created initially
func (pp *pcktProcess) Populate(algorithm string) {

	found := false
	lowerAlg := strings.ToLower(algorithm)

	for _, alg := range Algorithms {
		if lowerAlg == strings.ToLower(alg) {
			found = true
			break
		}
	}
	if !found {
		panic(fmt.Errorf("pcktProcess algorithm must be in %v", Algorithms))
	}
	pp.Algorithm = algorithm
}

// pcktProcessStart is called on entry to a function of the PcktProcess class.
func pcktProcessStart(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string,
	msg *CmpPtnMsg) (bool, bool, float64, []*CmpPtnMsg) {

	state := cpfi.State.(*pcktProcess)

	// delay is from decrypt, process, encrypt
	delay := FuncExecTime(cpfi, methodCode, "", msg)

	algOp := "encrypt-" + strings.ToLower(state.Algorithm)
	delay += FuncExecTime(cpfi, algOp, "", msg)

	algOp = "decrypt-" + strings.ToLower(state.Algorithm)
	delay += FuncExecTime(cpfi, algOp, "", msg)

	// a function of the PcktProcess class should have exactly one output for every input type
	ies := InEdgeMsg(msg)
	sfo := cpfi.Resp[ies][0]

	// update the edge label on the message
	UpdateMsg(msg, cpfi.Label, sfo.MsgType, sfo.DstLabel, "")

	return true, true, delay, []*CmpPtnMsg{msg}
}

// serialize transforms the pcktProcess into string form for
// inclusion through a file
func (pp *pcktProcess) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*pp)
	} else {
		bytes, merr = json.Marshal(*pp)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a serialized representation of a pcktProcess structure
func (pp *pcktProcess) Deserialize(pps string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	ppsb := []byte(pps)
	example := pcktProcess{ClassName: "name"}

	// Select whether we read in json or yaml
	if useYAML {
		err = yaml.Unmarshal(ppsb, &example)
	} else {
		err = json.Unmarshal(ppsb, &example)
	}

	if err != nil {
		return nil, err
	}
	return &example, nil
}
