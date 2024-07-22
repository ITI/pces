package mrnesbits

// file class.go holds structures, methods, functions, data structures, and event handlers
// related to the 'class' specialization of instances of computational functions

import (
	"encoding/json"
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
	"github.com/iti/mrnes"
	"gopkg.in/yaml.v3"
	"math"
	"strconv"
	"strings"
)

// A FuncClass represents the methods used to simulate the effect
// of executing a function,  different types of input generate different
// types of responses, so we use a map whose key selects the start, end pair of methods
type FuncClass interface {
	FuncClassName() string
	CreateState(string, bool) any
	InitState(*CmpPtnFuncInst, string, bool)
}

// StartMethod gives the signature of functions called to implement 
// a function's entry point
type StartMethod func(*evtm.EventManager, *CmpPtnFuncInst, string, *CmpPtnMsg)

// RespMethod associates two RespFunc that implement a function's response,
// one when it starts, the other when it ends
type RespMethod struct {
	Start StartMethod
	End   evtm.EventHandlerFunction
}

// FuncClassNames needs to have an indexing key for every class of function that
// might be included in a model.
var FuncClassNames map[string]bool = map[string]bool{"connSrc": true, "processPckt": true, "cryptoPckt": true, "cycleDst": true, "chgCP": true, "finish": true}

// RegisterFuncClass is called to tell the system that a particular
// function class exists, and gives a point to its description.
// The idea is to register only those function classes actually used, or at least,
// provide clear separation between classes.  Return of bool allows call to RegisterFuncClass
// as part of a variable assignment outside of a function body
func RegisterFuncClass(fc FuncClass) bool {
	className := fc.FuncClassName()
	_, present := FuncClasses[className]
	if present {
		panic(fmt.Errorf("attempt to register function class %s after that name already registered", fc.FuncClassName()))
	}
	FuncClasses[className] = fc
	return true
}

// FuncClasses is a map that takes us from the name of a FuncClass
var FuncClasses map[string]FuncClass = make(map[string]FuncClass)

// ClassMethods maps the function class name to a map indexed by (string) operation code to get to
// a pair of functions to handle the entry and exit to the function
var ClassMethods map[string]map[string]RespMethod = make(map[string]map[string]RespMethod)
var ClassMethodsBuilt bool = CreateClassMethods()

func CreateClassMethods() bool {
	// build table for connSrc class
	fmap := make(map[string]RespMethod)
	fmap["generateOp"] = RespMethod{Start: connSrcEnterStart, End: connSrcExitStart}
	fmap["completeOp"] = RespMethod{Start: connSrcEnterReturn, End: connSrcExitReturn}
	ClassMethods["connSrc"] = fmap

	fmap = make(map[string]RespMethod)
	fmap["generateOp"] = RespMethod{Start: cycleDstEnterStart, End: cycleDstExitStart}
	fmap["completeOp"] = RespMethod{Start: cycleDstEnterReturn, End: cycleDstExitReturn}
	ClassMethods["cycleDst"] = fmap

	// build table for processPckt class
	fmap = make(map[string]RespMethod)
	fmap["processOp"] = RespMethod{Start: processPcktEnter, End: processPcktExit}
	ClassMethods["processPckt"] = fmap

	// build table for cryptoPckt class
	fmap = make(map[string]RespMethod)
	fmap["cryptoOp"] = RespMethod{Start: cryptoPcktEnter, End: cryptoPcktExit}
	ClassMethods["cryptoPckt"] = fmap

	// build table for finish class
	fmap = make(map[string]RespMethod)
	fmap["finishOp"] = RespMethod{Start: finishEnter, End: ExitFunc}
	ClassMethods["finish"] = fmap

	// build table for chgCP class
	fmap = make(map[string]RespMethod)
	fmap["chgCP"] = RespMethod{Start: chgCPEnter, End: ExitFunc}
	ClassMethods["chgCP"] = fmap

	return true
}

func UpdateMsg(msg *CmpPtnMsg, srcCPID int, srcLabel, msgType, dstLabel string) {
	msg.MsgType = msgType
	msg.PrevCPID = srcCPID
	msg.Edge = *CreateCmpPtnGraphEdge(srcLabel, msgType, dstLabel, "")
}

func validFuncClass(class string) bool {
	_, present := FuncClasses[class]
	return present
}

//-------- methods and state for function class connSrc

var csrcVar *connSrc = ClassCreateConnSrc()
var connSrcLoaded bool = RegisterFuncClass(csrcVar)

type connSrc struct {
	ClassName               string
	InterarrivalDist        string // "random", "constant", "none"
	InterarrivalMean        float64
	InitMsgType             string
	InitMsgLen, InitPcktLen int
	InitiationLimit         int
	Calls                   int
	Returns                 int
	Trace                   bool
	OpName                  map[string]string
}

func (cs *connSrc) Populate(mean float64, limit int, interarrival, msgType string, msgLen, pcktLen, calls int, trace bool) {
	cs.Calls = calls
	cs.InitMsgLen = msgLen
	cs.InitPcktLen = pcktLen
	cs.InitMsgType = msgType
	cs.InterarrivalDist = interarrival
	cs.InterarrivalMean = mean
	cs.InitiationLimit = limit
	cs.Trace = trace
}

func ClassCreateConnSrc() *connSrc {
	cs := new(connSrc)
	cs.ClassName = "connSrc"
	cs.Calls = 0
	cs.Returns = 0
	cs.InitMsgType = ""
	cs.InitMsgLen = 0
	cs.InitPcktLen = 0

	// Interarrival{Dist,Mean} defaults
	cs.InterarrivalDist = "const"
	cs.InterarrivalMean = 0.0
	cs.InitiationLimit = 0
	cs.Trace = false
	cs.OpName = make(map[string]string)
	return cs
}

func (cs *connSrc) AddOpName(methodCode, opName string) {
	cs.OpName[methodCode] = opName
}

func (cs *connSrc) FuncClassName() string {
	return cs.ClassName
}

func (cs *connSrc) CreateState(stateStr string, useYAML bool) any {
	csVarAny, err := cs.Deserialize(stateStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("connSrc.InitState for %s sees deserialization error", cs.ClassName))
	}
	return csVarAny
}

// InitState saves the state encoded for the named cpfi, and copies
// the values given there for Interarrival{Dist, Mean} to the cpfi structure
func (cs *connSrc) InitState(cpfi *CmpPtnFuncInst, stateStr string, useYAML bool) {
	csVarAny := cs.CreateState(stateStr, useYAML)
	csv := csVarAny.(*connSrc)
	csv.OpName = map[string]string{"initiate": "generateOp"}
	cpfi.State = csv
	cpfi.trace = csv.Trace
	cpfi.InitFunc = connSrcSchedule
	cpfi.InitMsgParams(csv.InitMsgType, csv.InitMsgLen, csv.InitPcktLen, 0.0)
	cpfi.InterarrivalDist = csv.InterarrivalDist
	cpfi.InterarrivalMean = csv.InterarrivalMean
}

// connSrcSchedule schedules the next initiation at the comp pattern function instance pointed to
// by the context, and samples the next arrival time
func connSrcSchedule(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	cs := cpfi.State.(*connSrc)
	cpi := CmpPtnInstByName[cpfi.PtnName]

	css := cpfi.State.(*connSrc)

	// if we do initiate messages from the implicitly named cpfi, see whether
	// we have done already all the ones programmed to be launched
	if css.InitiationLimit > 0 && css.InitiationLimit <= css.Calls {
		return nil
	}

	// default interarrival time, if one is not otherwise chosen
	interarrival := float64(0.0)

	// introduce interarrival delay if an interarrival distribution is named
	if cs.InterarrivalDist != "none" {
		interarrival = csinterarrivalSample(cpi, cs.InterarrivalDist, cs.InterarrivalMean)
		evtMgr.Schedule(cpfi, nil, EnterFunc, vrtime.SecondsToTime(interarrival))
	} else {
		evtMgr.Schedule(cpfi, nil, EnterFunc, vrtime.CreateTime(1, 0))
	}

	// the absence of a message flags (along with cpfi.InitFunc not being empty)
	// tells EnterFunc to spawn a new message

	// skip interarrival generation of source generation if no interarrival distribution named.  Means it is generated elsewhere,
	// probably based on completion of some task
	if cs.InterarrivalDist != "none" {
		evtMgr.Schedule(cpfi, nil, connSrcSchedule, vrtime.SecondsToTime(interarrival))
	}
	return nil
}

// serialize transforms the connSrc into string form for
// inclusion through a file
func (cs *connSrc) Serialize(useYAML bool) (string, error) {
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

// Deserialize recovers a serialized representation of a connSrc structure
func (cs *connSrc) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)
	example := connSrc{Calls: 0, Returns: 0}

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

// connSrcEnterStart schedules the generation of a packet
func connSrcEnterStart(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string,
	msg *CmpPtnMsg) {

	state := cpfi.State.(*connSrc)
	state.Calls += 1

	// look up the generation service requirement
	genTime := FuncExecTime(cpfi, methodCode, msg)

	// call the host's scheduler.
	mrnes.TaskSchedulerByHostName[cpfi.host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, connSrcExitStart)
}

// connSrcExitStart executes at the time when the initiating connSrc activation completes
func connSrcExitStart(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	msg := data.(*CmpPtnMsg)

	_, present := cpfi.msgResp[msg.ExecID]
	if !present {
		// exitFunc will be looking at the msg edge label, expecting it to have been
		// updated for the next step.   The cpfi should have only one output edge
		msgType := cpfi.outEdges[0].MsgType
		dstLabel := cpfi.outEdges[0].FuncLabel
		UpdateMsg(msg, cpfi.CPID, cpfi.Label, msgType, dstLabel)

		// place the popped message where exitFunc will find it
		// change the edge on the message to reflect its next stop.
		// We get the destination label from the comp pattern instance graph, and the edge label by searching
		// the outEdges for the function
		cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})
	}

	// schedule the exitFunc handler
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

// connSrcEnterReturn deals with the arrival of a return from the original message
func connSrcEnterReturn(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {

	state := cpfi.State.(*connSrc)
	state.Returns += 1

	// look up the generation service requirement
	genTime := FuncExecTime(cpfi, methodCode, msg)

	// call the host's scheduler.

	// N.B. perhaps we can schedule ExitFunc here rather than
	mrnes.TaskSchedulerByHostName[cpfi.host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, connSrcExitReturn)
}

// connSrcEnd executes at the completion of the return processing delay.
// Do we need to make the output message slice available here?
func connSrcExitReturn(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	msg := data.(*CmpPtnMsg)

	// there are no messages to forward past this function exit.
	// ExitFunc will look to see what it needs to forward
	cpfi.msgResp[msg.ExecID] = []*CmpPtnMsg{}

	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

//-------- methods and state for function class cycleDst

var cdstVar *cycleDst = ClassCreateCycleDst()
var cycleDstLoaded bool = RegisterFuncClass(cdstVar)

type cycleDst struct {
	ClassName string
	PcktDist  string
	PcktMu    float64

	BurstDist string
	BurstMu   float64
	BurstLen  int

	CycleDist string
	CycleMu   float64
	EUDCycles int

	InitMsgType             string
	InitMsgLen, InitPcktLen int
	Dsts                    int
	BaseDstName             string

	CycleID int
	BurstID int
	DstID   int

	Calls   int
	Returns int
	Trace   bool
	OpName  map[string]string
}

func (cd *cycleDst) Populate(dsts int, baseDstName string,
	pcktDist string, pcktMu float64, burstDist string, burstMu float64, pcktBurst int,
	cycleDist string, cycleMu float64, eudCycles int,
	msgLen, pcktSize int, trace bool) {

	cd.ClassName = "cycleDst"
	cd.PcktDist = pcktDist
	cd.PcktMu = pcktMu

	cd.BurstDist = burstDist
	cd.BurstMu = burstMu
	cd.BurstLen = pcktBurst

	cd.CycleDist = cycleDist
	cd.CycleMu = cycleMu
	cd.EUDCycles = eudCycles

	cd.InitMsgLen = msgLen
	cd.InitPcktLen = pcktSize
	cd.InitMsgType = "initiate"

	cd.Dsts = dsts
	cd.BaseDstName = baseDstName

	cd.Trace = trace
	cd.OpName = make(map[string]string)

	cd.Dsts = dsts
	cd.Trace = trace
}

func ClassCreateCycleDst() *cycleDst {
	cd := new(cycleDst)
	cd.ClassName = "cycleDst"
	return cd
}

func (cd *cycleDst) AddOpName(methodCode, opName string) {
	cd.OpName[methodCode] = opName
}

func (cd *cycleDst) FuncClassName() string {
	return cd.ClassName
}

func (cd *cycleDst) CreateState(stateStr string, useYAML bool) any {
	cdVarAny, err := cd.Deserialize(stateStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("cycleDst.InitState for %s sees deserialization error", cd.ClassName))
	}
	return cdVarAny
}

// InitState saves the state encoded for the named cpfi, and copies
// the values given there for Interarrival{Dist, Mean} to the cpfi structure
func (cd *cycleDst) InitState(cpfi *CmpPtnFuncInst, stateStr string, useYAML bool) {
	cdVarAny := cd.CreateState(stateStr, useYAML)
	cdv := cdVarAny.(*cycleDst)
	cdv.OpName = map[string]string{"initiate": "generateOp"}
	cpfi.State = cdv
	cpfi.trace = cdv.Trace
	cpfi.InitFunc = cycleDstSchedule

	cpfi.InitMsgParams(cdv.InitMsgType, cdv.InitMsgLen, cdv.InitPcktLen, 0.0)
}

// scheduleBurst schedules the immediate generation of as many
// packts for the target EUD as are configured.
func scheduleBurst(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, cpm *CmpPtnMsg, after float64) {
	cds := cpfi.State.(*cycleDst)
	for idx := 0; idx < cds.BurstLen; idx++ {
		newMsg := cpm
		newMsg.Start = true // when this message first processed, start the timer
		if idx > 0 {
			newMsg = new(CmpPtnMsg)
			*newMsg = *cpm
			newMsg.ExecID = numExecThreads
			numExecThreads += 1
		}
		if cpfi == nil {
			panic(fmt.Errorf("empty cpfi"))
		}
		evtMgr.Schedule(cpfi, newMsg, EnterFunc, vrtime.SecondsToTime(after))
	}
}

// cycleDstSchedule schedules the next burst.
func cycleDstSchedule(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	cds := cpfi.State.(*cycleDst)
	cpi := CmpPtnInstByName[cpfi.PtnName]

	// default interarrival time for bursts, if one is not otherwise chosen
	interarrival := float64(0.0)
	if cds.PcktDist == "exp" {
		interarrival = csinterarrivalSample(cpi, cds.PcktDist, cds.PcktMu)
	} else {
		interarrival = cds.PcktMu
	}

	cpm := new(CmpPtnMsg)
	*cpm = *cpfi.InitMsg
	cpm.ExecID = numExecThreads
	cpm.MsgType = "initiate"
	numExecThreads += 1
	cpm.SrcCP = cpi.name
	cpm.DstCP = cds.BaseDstName + "-" + strconv.Itoa(cds.BurstID%cds.Dsts)
	cpm.NxtCP = ""

	// launch the burst
	scheduleBurst(evtMgr, cpfi, cpm, interarrival)

	// if we have shot off a burst for every destination we are done
	if (cds.BurstID%cds.Dsts == (cds.Dsts - 1)) && (cds.CycleID == cds.EUDCycles) {
		return nil
	}

	cds.BurstID += 1
	if cds.BurstID%cds.Dsts == 0 {
		cds.CycleID += 1
	}

	// schedule the next burst schedule to occur at the same time (but after)
	// the burst just scheduled
	evtMgr.Schedule(cpfi, nil, cycleDstSchedule, vrtime.SecondsToTime(interarrival))

	return nil
}

// serialize transforms the cycleDst into string form for
// inclusion through a file
func (cd *cycleDst) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*cd)
	} else {
		bytes, merr = json.Marshal(*cd)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a serialized representation of a cycleDst structure
func (cd *cycleDst) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)
	example := cycleDst{Calls: 0, Returns: 0}

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

// cycleDstEnterStart schedules the generation of a packet
func cycleDstEnterStart(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string,
	msg *CmpPtnMsg) {

	state := cpfi.State.(*cycleDst)
	state.Calls += 1

	// look up the generation service requirement
	genTime := FuncExecTime(cpfi, methodCode, msg)

	// call the host's scheduler.
	mrnes.TaskSchedulerByHostName[cpfi.host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, cycleDstExitStart)
}

// cycleDstExitStart executes at the time when the initiating cycleDst activation completes
func cycleDstExitStart(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	msg := data.(*CmpPtnMsg)

	_, present := cpfi.msgResp[msg.ExecID]
	if !present {
		// exitFunc will be looking at the msg edge label, expecting it to have been
		// updated for the next step.   The cpfi should have only one output edge
		msgType := cpfi.outEdges[0].MsgType
		dstLabel := cpfi.outEdges[0].FuncLabel
		UpdateMsg(msg, cpfi.CPID, cpfi.Label, msgType, dstLabel)

		// place the popped message where exitFunc will find it
		// change the edge on the message to reflect its next stop.
		// We get the destination label from the comp pattern instance graph, and the edge label by searching
		// the outEdges for the function
		cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})
	}

	// schedule the exitFunc handler
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

// cycleDstEnterReturn deals with the arrival of a return from the original message
func cycleDstEnterReturn(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {

	state := cpfi.State.(*cycleDst)
	state.Returns += 1

	// look up the generation service requirement
	genTime := FuncExecTime(cpfi, methodCode, msg)

	// call the host's scheduler.
	mrnes.TaskSchedulerByHostName[cpfi.host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, cycleDstExitReturn)
}

// cycleDstEnd executes at the completion of packet generation
func cycleDstExitReturn(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	msg := data.(*CmpPtnMsg)

	cpfi.msgResp[msg.ExecID] = []*CmpPtnMsg{msg}

	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

func csinterarrivalSample(cpi *CmpPtnInst, dist string, mean float64) float64 {
	switch dist {
	case "const":
		return mean
	case "exp":
		u := cpi.Rngs.RandU01()
		return -1.0 * mean * math.Log(u)
	}
	return mean
}

//-------- methods and state for function class processPckt

var ppVar *processPckt = ClassCreateProcessPckt()
var processPcktLoaded bool = RegisterFuncClass(ppVar)

type processPckt struct {
	ClassName string
	OpName    map[string]string
	Calls     int
	Return    bool
	Trace     bool
}

func ClassCreateProcessPckt() *processPckt {
	pp := new(processPckt)
	pp.ClassName = "processPckt"
	pp.OpName = make(map[string]string)
	pp.Calls = 0
	pp.Return = false
	pp.Trace = false
	return pp
}

func (pp *processPckt) FuncClassName() string {
	return pp.ClassName
}

func (pp *processPckt) CreateState(stateStr string, useYAML bool) any {
	ppVarAny, err := pp.Deserialize(stateStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("processPckt.InitState for %s sees deserialization error", pp.ClassName))
	}
	return ppVarAny
}

func (pp *processPckt) InitState(cpfi *CmpPtnFuncInst, stateStr string, useYAML bool) {
	ppVarAny := pp.CreateState(stateStr, useYAML)
	ppv := ppVarAny.(*processPckt)
	cpfi.State = ppv
	cpfi.trace = ppv.Trace
}

// serialize transforms the processPckt into string form for
// inclusion through a file
func (pp *processPckt) Serialize(useYAML bool) (string, error) {
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

// Deserialize recovers a serialized representation of a processPckt structure
func (pp *processPckt) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)
	example := processPckt{Calls: 0}

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

// processPcktEnter schedules the simulation of processing one packet
func processPcktEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {

	state := cpfi.State.(*processPckt)
	opName, present := state.OpName[methodCode]
	if !present {
		panic(fmt.Errorf("method code %s not represented in OpName map", methodCode))
	}
	state.Calls += 1

	// look up the generation service requirement.
	genTime := FuncExecTime(cpfi, opName, msg)

	// call the host's scheduler.
	mrnes.TaskSchedulerByHostName[cpfi.host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, processPcktExit)
}

// processPcktExit executes when the associated message did not get served immediately on being scheduled,
// but now has finished.
func processPcktExit(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	msg := data.(*CmpPtnMsg)
	state := cpfi.State.(*processPckt)

	// exitFunc will be looking at the msg edge label, expecting it to have been
	// updated for the next step
	//
	msgType := cpfi.outEdges[0].MsgType
	dstLabel := cpfi.outEdges[0].FuncLabel
	UpdateMsg(msg, cpfi.CPID, cpfi.Label, msgType, dstLabel)

	// put the response where ExitFunc will find it
	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})

	// if the Return bit is set, reverse the SrcCP and DstCP elements of msg
	if state.Return {
		tmp := msg.SrcCP
		msg.SrcCP = msg.DstCP
		msg.DstCP = tmp
	}
	// schedule the exitFunc handler
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

//-------- methods and state for function class cryptoPckt

var cpVar *cryptoPckt = ClassCreateCryptoPckt()
var cryptoPcktLoaded bool = RegisterFuncClass(cpVar)

type cryptoPckt struct {
	ClassName string
	Op        string // "encrypt", "decrypt", "hash", "sign", etc
	Algorithm string // aes, des, etc
	KeyLength int
	OpCode    string
	Calls     int
	Return    bool
	Trace     bool
}

func ClassCreateCryptoPckt() *cryptoPckt {
	cp := new(cryptoPckt)
	cp.ClassName = "cryptoPckt"
	return cp
}

func (cp *cryptoPckt) Populate(op, alg, keylength string) {
	cp.Op = strings.ToLower(op)
	cp.Algorithm = strings.ToLower(alg)
	cp.OpCode = cp.Op + "-" + cp.Algorithm + "-" + keylength
	cp.KeyLength, _ = strconv.Atoi(keylength)
}

func (cp *cryptoPckt) FuncClassName() string {
	return "cryptoPckt"
}

func (cp *cryptoPckt) CreateState(stateStr string, useYAML bool) any {
	cpVarAny, err := cp.Deserialize(stateStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("cryptoPckt.InitState for %s sees deserialization error", cp.ClassName))
	}
	return cpVarAny
}

func (cp *cryptoPckt) InitState(cpfi *CmpPtnFuncInst, stateStr string, useYAML bool) {
	cpVarAny := cp.CreateState(stateStr, useYAML)
	cpv := cpVarAny.(*cryptoPckt)
	cpfi.State = cpv
	cpfi.trace = cpv.Trace
}

// serialize transforms the cryptoPckt into string form for
// inclusion through a file
func (cp *cryptoPckt) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*cp)
	} else {
		bytes, merr = json.Marshal(*cp)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a serialized representation of a cryptoPckt structure
func (cp *cryptoPckt) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)
	example := cryptoPckt{Calls: 0}

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

// cryptoPcktEnter schedules the simulation of processing one packet
func cryptoPcktEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {

	state := cpfi.State.(*cryptoPckt)
	state.Calls += 1

	// look up the generation service requirement.
	genTime := FuncExecTime(cpfi, state.OpCode, msg)

	// call the host's scheduler.
	mrnes.TaskSchedulerByHostName[cpfi.host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, cryptoPcktExit)
}

// cryptoPcktExit executes when the associated message did not get served immediately on being scheduled,
// but now has finished.
func cryptoPcktExit(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	msg := data.(*CmpPtnMsg)
	state := cpfi.State.(*cryptoPckt)

	// exitFunc will be looking at the msg edge label, expecting it to have been
	// updated for the next step
	//
	msgType := cpfi.outEdges[0].MsgType
	dstLabel := cpfi.outEdges[0].FuncLabel
	UpdateMsg(msg, cpfi.CPID, cpfi.Label, msgType, dstLabel)

	// put the response where ExitFunc will find it
	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})

	// if the Return bit is set, reverse the SrcCP and DstCP elements of msg
	if state.Return {
		tmp := msg.SrcCP
		msg.SrcCP = msg.DstCP
		msg.DstCP = tmp
	}
	// schedule the exitFunc handler
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

//-------- methods and state for function class chgCP

var ccpVar *chgCP = ClassCreateChgCP()
var chgCPLoaded bool = RegisterFuncClass(ccpVar)

type ChgDesc struct {
	CP, Label, MsgType string
}

func CreateChgDesc(cp, label, msgType string) ChgDesc {
	return ChgDesc{CP: cp, Label: label, MsgType: msgType}
}

type chgCP struct {
	ClassName string
	Calls     int
	ChgMap    map[string]ChgDesc
	Trace     bool
}

func ClassCreateChgCP() *chgCP {
	ccp := new(chgCP)
	ccp.ClassName = "chgCP"
	ccp.Calls = 0
	ccp.ChgMap = make(map[string]ChgDesc)
	ccp.Trace = false
	return ccp
}

func (ccp *chgCP) FuncClassName() string {
	return ccp.ClassName
}

func (ccp *chgCP) CreateState(stateStr string, useYAML bool) any {
	ccpVarAny, err := ccp.Deserialize(stateStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("chgCP.InitState for %s sees deserialization error", ccp.ClassName))
	}
	return ccpVarAny
}

func (ccp *chgCP) InitState(cpfi *CmpPtnFuncInst, stateStr string, useYAML bool) {
	ccpVarAny := ccp.CreateState(stateStr, useYAML)
	ccpv := ccpVarAny.(*chgCP)
	cpfi.State = ccpv
	cpfi.trace = ccpv.Trace
}

// serialize transforms the chgCP into string form for
// inclusion through a file
func (ccp *chgCP) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*ccp)
	} else {
		bytes, merr = json.Marshal(*ccp)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a serialized representation of a chgCP structure
func (ccp *chgCP) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)
	example := chgCP{Calls: 0}

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

// chgCPEnter schedules the simulation of processing one packet
func chgCPEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {

	state := cpfi.State.(*chgCP)
	state.Calls += 1

	// modify message to state it should have on presentation to the next comp pattern
	chg := state.ChgMap[msg.DstCP]

	// to which CP should we change?
	msg.NxtCP = chg.CP

	// modify the message to reflect the transformation
	UpdateMsg(msg, cpfi.CPID, cpfi.Label, chg.MsgType, chg.Label)

	// leave the transformed message where ExitFunc will find it
	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))

}

//-------- methods and state for function class finish

var fnshVar *finish = ClassCreateFinish()
var finishLoaded bool = RegisterFuncClass(fnshVar)

type finish struct {
	ClassName string
	Calls     int
	Trace     bool
}

func ClassCreateFinish() *finish {
	fnsh := new(finish)
	fnsh.ClassName = "finish"
	fnsh.Calls = 0
	fnsh.Trace = false
	return fnsh
}

func (fnsh *finish) FuncClassName() string {
	return fnsh.ClassName
}

func (fnsh *finish) CreateState(stateStr string, useYAML bool) any {
	fnshVarAny, err := fnsh.Deserialize(stateStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("finish.InitState for %s sees deserialization error", fnsh.ClassName))
	}
	return fnshVarAny
}

func (fnsh *finish) InitState(cpfi *CmpPtnFuncInst, stateStr string, useYAML bool) {
	fnshVarAny := fnsh.CreateState(stateStr, useYAML)
	fnshv := fnshVarAny.(*finish)
	cpfi.State = fnshv
	cpfi.trace = fnshv.Trace
}

// serialize transforms the finish into string form for
// inclusion through a file
func (fnsh *finish) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*fnsh)
	} else {
		bytes, merr = json.Marshal(*fnsh)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a serialized representation of a finish structure
func (fnsh *finish) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)
	example := finish{Calls: 0}

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

// finishEnter flags
func finishEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {

	state := cpfi.State.(*finish)
	state.Calls += 1

	// return empty messages to flag completion
	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{})
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
}
