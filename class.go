package pces

// file class.go holds structures, methods, functions, data structures, and event handlers
// related to the 'class' specialization of instances of computational functions

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
	"github.com/iti/mrnes"
	"gopkg.in/yaml.v3"
	"encoding/json"
	"math"
	"strings"
)

// FuncClassCfg represents the methods used to simulate the effect
// of executing a function,  different types of input generate different
// types of responses, so we use a map whose key selects the start, end pair of methods
type FuncClassCfg interface {
	FuncClassName() string
	CreateCfg(string) any
	InitCfg(*evtm.EventManager, *CmpPtnFuncInst, string, bool)
	ValidateCfg(*CmpPtnFuncInst) error
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
// might be included in a model.  Map exists just for checking validity of
// a function class name when building a model.   Any new class introduced
// in this file needs to have an entry placed in this dictionary.
var FuncClassNames map[string]bool = map[string]bool{
	// "connSrc": true,
	"measure":     true,
	"processPckt": true,
	"srvRsp":      true,
	"srvReq":      true,
	"transfer":    true,
	"start":       true,
	"finish":      true,
	"bckgrndLd":   true}

// RegisterFuncClass is called to tell the system that a particular
// function class exists, and gives a point to its description.
// The idea is to register only those function classes actually used, or at least,
// provide clear separation between classes.  Reflect of bool allows call to RegisterFuncClass
// as part of a variable assignment outside of a function body
func RegisterFuncClass(fc FuncClassCfg) bool {
	className := fc.FuncClassName()
	_, present := FuncClasses[className]
	if present {
		return true
	}
	FuncClasses[className] = fc
	FuncClassNames[className] = true
	return true
}

// FuncClasses is a map that takes us from the name of a FuncClass
var FuncClasses map[string]FuncClassCfg = make(map[string]FuncClassCfg)

// HndlrMap maps a string name for an event handling function to the function itself
var HndlrMap map[string]evtm.EventHandlerFunction = map[string]evtm.EventHandlerFunction{"empty": EmptyInitFunc}

// AddHndlrMap includes a new association between string and event handler
func AddHndlrMap(name string, hndlr evtm.EventHandlerFunction) {
	if HndlrMap == nil {
		HndlrMap = make(map[string]evtm.EventHandlerFunction)
	}
	HndlrMap[name] = hndlr
}

// ClassMethods maps the function class name to a map indexed by (string) operation code to get to
// a pair of functions to handle the entry and exit to the function
var ClassMethods map[string]map[string]RespMethod = make(map[string]map[string]RespMethod)
var ClassMethodsBuilt bool = CreateClassMethods()

// CreateClassMethods fills in the ClassMethods table for each of the defined function classes
func CreateClassMethods() bool {
	// ClassMethod key is the string identity of a function class, points to
	// a map whose key is a methodCode.

	// build table for processPckt class
	fmap := make(map[string]RespMethod)
	fmap["default"] = RespMethod{Start: processPcktEnter, End: processPcktExit}
	fmap["processOp"] = RespMethod{Start: processPcktEnter, End: processPcktExit}
	ClassMethods["processPckt"] = fmap

	// build table for start class
	fmap = make(map[string]RespMethod)
	fmap["default"] = RespMethod{Start: startEnter, End: ExitFunc}
	ClassMethods["start"] = fmap

	// build table for finish class
	fmap = make(map[string]RespMethod)
	fmap["default"] = RespMethod{Start: finishEnter, End: ExitFunc}
	fmap["finishOp"] = RespMethod{Start: finishEnter, End: ExitFunc}
	ClassMethods["finish"] = fmap

	fmap = make(map[string]RespMethod)
	fmap["default"] = RespMethod{Start: measureEnter, End: ExitFunc}
	fmap["measure"] = RespMethod{Start: measureEnter, End: ExitFunc}
	ClassMethods["measure"] = fmap

	fmap = make(map[string]RespMethod)
	fmap["default"] = RespMethod{Start: srvRspEnter, End: ExitFunc}
	ClassMethods["srvRsp"] = fmap

	fmap = make(map[string]RespMethod)
	fmap["default"] = RespMethod{Start: srvReqEnter, End: ExitFunc}
	fmap["request"] = RespMethod{Start: srvReqEnter, End: ExitFunc}
	fmap["return"] = RespMethod{Start: srvReqRtn, End: ExitFunc}
	ClassMethods["srvReq"] = fmap

	fmap = make(map[string]RespMethod)
	fmap["default"] = RespMethod{Start: transferEnter, End: ExitFunc}
	ClassMethods["transfer"] = fmap
	// need to have bckgrndLd as a key to ClassMethods, but it
	// doesn't use the RespMethod mechanisms
	ClassMethods["bckgrndLd"] = make(map[string]RespMethod)
	return true
}

func checkMCValidity(funcClass string, mc string) bool {
	fcm, present := ClassMethods[funcClass]
	if !present {
		return false
	}
	_, present = fcm[mc]
	return present
}

func CreateNewClass(className string) {
	fmap := make(map[string]RespMethod)
	ClassMethods[className] = fmap
}

func AddClassMethod(className string, methodName string, rm RespMethod) {
	ClassMethods[className][methodName] = rm
}

// UpdateMsg copies the pattern, label coordinates of the message's current
// position to be the previous one and labels the next coordinates with
// the values given as arguments
func UpdateMsg(msg *CmpPtnMsg, nxtCPID int, nxtLabel, msgType string) {
	msg.MsgType = msgType
	msg.CPID = nxtCPID
	msg.Label = nxtLabel
}

// validFunctionClass checks whether the string argument is a function class
// name that appears as an index in the FuncClasses map to an entity that
// meets the FuncClassCfg interface
func validFuncClass(class string) bool {
	_, present := FuncClasses[class]
	return present
}

func advanceMsg(cpfi *CmpPtnFuncInst, msg *CmpPtnMsg, msgTypeOut string) *CmpPtnMsg {
	var nxtCPID int
	var nxtMsgType string
	var nxtLabel string

	var eeidx int
	var present bool

	// if there is only one outedge we'll choose that. Throw a warning if 
	// the msgType passed in differs from the msgType on the edge 
	if len(cpfi.OutEdges) == 1 {
		eeidx = 0
		nxtMsgType = cpfi.OutEdges[0].MsgType
	} else if len(msgTypeOut) > 0 {
		// Get the index of the OutEdge associated with the passed-in msgType.
		eeidx, present = cpfi.Msg2Idx[msgTypeOut]
		if !present {
			panic(fmt.Errorf("expected output edge for funcion %s", cpfi.Label))
		}
		nxtMsgType = cpfi.OutEdges[eeidx].MsgType
	}

	// pull the next message's type, computation pattern, and label of target function from
	// the outedge structure
	nxtLabel = cpfi.OutEdges[eeidx].FuncLabel
	nxtCPID = cpfi.OutEdges[eeidx].CPID

	// modify the message in place
	UpdateMsg(msg, nxtCPID, nxtLabel, nxtMsgType)

	// put the response where ExitFunc will find it
	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})
	return msg
}

func copyDict(dict1, dict2 map[string]string) {
	for key, value := range dict2 {
		dict1[key] = value
	}
}

//-------- methods and state for function class processPckt

var ppVar *processPcktCfg = ClassCreateProcessPcktCfg()
var processPcktLoaded bool = RegisterFuncClass(ppVar)

type processPcktCfg struct {
	// map method code to an operation for the timing lookup
	TimingCode map[string]string `yaml:"timingcode" json:"timingcode"`
	Msg2MC     map[string]string `yaml:"msg2mc" json:"msg2mc"`
	Msg2Msg    map[string]string `yaml:"msg2msg" json:"msg2msg"`

	// if the packet is processed through an accelerator, its name in the destination endpoint
	AccelName string `yaml:"accelname" json:"accelname"`
	Trace     int    `yaml:"trace" json:"trace"`
}

type processPcktState struct {
	msgTypeIn string
	calls int
}

// ClassCreateProcessPcktCfg is a constructor called just to create an instance, fields unimportant
func ClassCreateProcessPcktCfg() *processPcktCfg {
	pp := new(processPcktCfg)
	pp.TimingCode = make(map[string]string)
	pp.Msg2MC = make(map[string]string)
	pp.Msg2Msg = make(map[string]string)
	pp.AccelName = ""
	pp.Trace = 0
	return pp
}

func createProcessPcktState() *processPcktState {
	pps := new(processPcktState)
	pps.calls = 0
	return pps
}

func (pp *processPcktCfg) FuncClassName() string {
	return "processPckt"
}

func (pp *processPcktCfg) CreateCfg(cfgStr string) any {
	useYAML := (cfgStr[0] != '{')
	ppVarAny, err := pp.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("processPckt.InitCfg sees deserialization error"))
	}
	return ppVarAny
}

func (pp *processPcktCfg) InitCfg(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	ppVarAny := pp.CreateCfg(cfgStr)
	ppv := ppVarAny.(*processPcktCfg)
	cpfi.Cfg = ppv
	cpfi.State = createProcessPcktState()
	copyDict(cpfi.Msg2MC, ppv.Msg2MC)
	cpfi.Trace = (ppv.Trace != 0)
}

func (pp *processPcktCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	return nil
}

// Serialize transforms the processPckt into string form for
// inclusion through a file
func (pp *processPcktCfg) Serialize(useYAML bool) (string, error) {
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

func (pp *processPcktCfg) CfgStr() string {
	rtn, err := pp.Serialize(true)
	if err != nil {
		panic(fmt.Errorf("process packet cfg serialization error"))
	}
	return rtn
}

// Deserialize recovers a serialized representation of a processPckt structure
func (pp *processPcktCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	tc := make(map[string]string)

	example := processPcktCfg{TimingCode: tc, Trace: 0}

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

// processPcktEnter schedules the simulation of processing one packet.
// Default handler
func processPcktEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {
	pps := cpfi.State.(*processPcktState)
	ppc := cpfi.Cfg.(*processPcktCfg)
	pps.calls += 1

	// determine whether an accelerator call is part of this cpfi and if so get the right time and scheduler
	var genTime float64
	var scheduler *mrnes.TaskScheduler

	pps.msgTypeIn = msg.MsgType
	opCode := msg.MsgType

	if len(ppc.AccelName) > 0 {
		// look up the model associated with this name
		genTime = AccelFuncExecTime(cpfi, ppc.AccelName, ppc.TimingCode[opCode], msg)
		scheduler = mrnes.AccelSchedulersByHostName[cpfi.Host][ppc.AccelName]
	} else {
		// not an accelerator call. look up the generation service requirement.
		genTime = HostFuncExecTime(cpfi, ppc.TimingCode[opCode], msg)
		scheduler = mrnes.TaskSchedulerByHostName[cpfi.Host]
	}

	endPtID := mrnes.EndptDevByName[cpfi.Host].DevID()
	execID := msg.ExecID

	// call the scheduler.
	scheduler.Schedule(evtMgr, methodCode, genTime, cpfi.Priority, math.MaxFloat64, cpfi, msg, execID, endPtID, processPcktExit)
}

// processPcktExit executes when the associated message did not get served immediately on being scheduled,
// but now has finished.
func processPcktExit(evtMgr *evtm.EventManager, context any, data any) any {

	cpfi := context.(*CmpPtnFuncInst)
	task := data.(*mrnes.Task)
	ppc := cpfi.Cfg.(*processPcktCfg)
	pps := cpfi.State.(*processPcktState)

	// get transformed message type as function of inbound message type
	outMsgType  := ppc.Msg2Msg[pps.msgTypeIn]	
	msg := advanceMsg(cpfi, task.Msg.(*CmpPtnMsg), outMsgType)

	// schedule the exitFunc handler
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

//-------- methods and state for function class start

var srtVar *StartCfg = ClassCreateStartCfg()
var startLoaded bool = RegisterFuncClass(srtVar)

type startState struct {
	calls int
}

type StartCfg struct {
	PcktLen   int               `yaml:"pcktlen" json:"pcktlen"`
	MsgLen    int               `yaml:"msglen" json:"msglen"`
	MsgType   string            `yaml:"msgtype" json:"msgtype"`
	Data      string            `yaml:"data" json:"data"`
	StartTime float64           `yaml:"starttime" json:"starttime"`
	Msg2MC    map[string]string `yaml:"msg2mc" json:"msg2mc"`
	Trace     int               `yaml:"trace" json:"trace"`
}

func ClassCreateStartCfg() *StartCfg {
	srt := new(StartCfg)
	srt.Msg2MC = make(map[string]string)
	srt.Trace = 0
	return srt
}

func createStartState() *startState {
	srt := new(startState)
	return srt
}

func (srt *StartCfg) FuncClassName() string {
	return "start"
}

func (srt *StartCfg) CreateCfg(cfgStr string) any {
	useYAML := cfgStr[0] != '{'
	srtVarAny, err := srt.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("start.InitCfg sees deserialization error"))
	}
	return srtVarAny
}

func (srt *StartCfg) InitCfg(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	srtVarAny := srt.CreateCfg(cfgStr)
	srtv := srtVarAny.(*StartCfg)
	cpfi.Cfg = srtv
	copyDict(cpfi.Msg2MC, srtv.Msg2MC)
	cpfi.State = createStartState()
	cpfi.Trace = (srtv.Trace != 0)
}

func (srt *StartCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	return nil
}

// Serialize transforms the start into string form for
// inclusion through a file
func (srt *StartCfg) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*srt)
	} else {
		bytes, merr = json.Marshal(*srt)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

func (srt *StartCfg) CfgStr() string {
	rtn, err := srt.Serialize(true)
	if err != nil {
		panic(fmt.Errorf("start cfg serialization error"))
	}
	return rtn
}

// Deserialize recovers a serialized representation of a start structure
func (srt *StartCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	example := StartCfg{Trace: 0}

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

// start an execution thread, main thing here is creating the initial
// message and giving it an execID
func startEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {
	srtg := cpfi.Cfg.(*StartCfg)
	srts := cpfi.State.(*startState)
	srts.calls += 1

	cpm := new(CmpPtnMsg)
	cpm.PcktLen = srtg.PcktLen
	cpm.MsgLen = srtg.MsgLen
	cpm.MsgType = srtg.MsgType
	numExecThreads += 1
	cpm.ExecID = numExecThreads

	endptName := cpfi.Host
	endpt := mrnes.EndptDevByName[endptName]
	AddCPTrace(traceMgr, evtMgr.CurrentTime(), cpm.ExecID, endpt.DevID(), "enter", cpm)

	srtTime := srtg.StartTime

	// out edge destination a function of the message type
	cpm = advanceMsg(cpfi, cpm, srtg.MsgType)

	cpfi.AddResponse(cpm.ExecID, []*CmpPtnMsg{cpm})
	evtMgr.Schedule(cpfi, cpm, ExitFunc, vrtime.SecondsToTime(srtTime))
}

//-------- methods and state for function class start

var fnshVar *FinishCfg = ClassCreateFinishCfg()
var finishLoaded bool = RegisterFuncClass(fnshVar)

type finishState struct {
	calls int
}

type FinishCfg struct {
	Msg2MC map[string]string `yaml:"msg2mc" json:"msg2mc"`
	Trace  int               `yaml:"trace" json:"trace"`
}

func ClassCreateFinishCfg() *FinishCfg {
	fnsh := new(FinishCfg)
	fnsh.Trace = 0
	fnsh.Msg2MC = make(map[string]string)
	return fnsh
}

func createFinishState() *finishState {
	fnsh := new(finishState)
	return fnsh
}

func (fnsh *FinishCfg) FuncClassName() string {
	return "finish"
}

func (fnsh *FinishCfg) CreateCfg(cfgStr string) any {
	useYAML := cfgStr[0] != '{'
	fnshVarAny, err := fnsh.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("finish.InitCfg sees deserialization error"))
	}
	return fnshVarAny
}

func (fnsh *FinishCfg) InitCfg(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	fnshVarAny := fnsh.CreateCfg(cfgStr)
	fnshv := fnshVarAny.(*FinishCfg)
	cpfi.Cfg = fnshv
	copyDict(cpfi.Msg2MC, fnshv.Msg2MC)
	cpfi.State = createFinishState()
	cpfi.Trace = (fnshv.Trace != 0)
}

func (fnsh *FinishCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	return nil
}

// Serialize transforms the finish into string form for
// inclusion through a file
func (fnsh *FinishCfg) Serialize(useYAML bool) (string, error) {
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

func (fnsh *FinishCfg) CfgStr() string {
	rtn, err := fnsh.Serialize(true)
	if err != nil {
		panic(fmt.Errorf("finish cfg serialization error"))
	}
	return rtn
}

// Deserialize recovers a serialized representation of a finish structure
func (fnsh *FinishCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	example := FinishCfg{Trace: 0}

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

// finishEnter drops information in the logs, and by not scheduling anything lets the execution thread finish
func finishEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {
	fns := cpfi.State.(*finishState)
	fns.calls += 1

	endptName := cpfi.Host
	endpt := mrnes.EndptDevByName[endptName]
	AddCPTrace(traceMgr, evtMgr.CurrentTime(), msg.ExecID, endpt.DevID(), "exit", msg)
}

// -------- methods and state for function class srvRsp
// srvRsp receives a request to perform some service, from
// a different comp pattern and label.  Those are found in the message's
// PrevCPID, prevLabel, and the requesting message type is
// in prevMsgType.   The srvRsp function delays based on the (configured) service operation
// and then responds back to the requesting source.
var srvRspVar *srvRspCfg = ClassCreateSrvRspCfg()
var srvRspLoaded bool = RegisterFuncClass(srvRspVar)

type srvRspState struct {
	calls int
}

type srvRspCfg struct {
	// chosen op, for timing
	TimingCode   map[string]string `yaml:"timingcode" json:"timingcode"`
	Msg2MC       map[string]string `yaml:"msg2mc" json:"msg2mc"`
	DirectPrefix []string          `yaml:"directprefix" json:"directprefix"`
	Trace        int               `yaml:"trace" json:"trace"`
}

func ClassCreateSrvRspCfg() *srvRspCfg {
	srvRsp := new(srvRspCfg)
	srvRsp.TimingCode = make(map[string]string)
	srvRsp.DirectPrefix = make([]string, 0)
	srvRsp.Msg2MC = make(map[string]string)
	srvRsp.Trace = 0
	return srvRsp
}

func createSrvRspState() *srvRspState {
	srvRsp := new(srvRspState)
	return srvRsp
}

func (srvRsp *srvRspCfg) FuncClassName() string {
	return "srvRsp"
}

func (srvRsp *srvRspCfg) CreateCfg(cfgStr string) any {
	useYAML := (cfgStr[0] != '{')
	srvRspVarAny, err := srvRsp.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("srvRsp.InitCfg sees deserialization error"))
	}
	return srvRspVarAny
}

func (srvRsp *srvRspCfg) Populate(tc map[string]string, trace bool) {
	for key, value := range tc {
		srvRsp.TimingCode[key] = value
	}
}

func (srvRsp *srvRspCfg) InitCfg(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	srvRspVarAny := srvRsp.CreateCfg(cfgStr)
	srvRspv := srvRspVarAny.(*srvRspCfg)
	cpfi.Cfg = srvRspv
	copyDict(cpfi.Msg2MC, srvRspv.Msg2MC)
	cpfi.IsService = true
	cpfi.State = createSrvRspState()

	// the srventication response function is a service, meaning
	// that its selection doesn't require identification of the source CP
	cpfi.Trace = (srvRspv.Trace != 0)
}

func (srvRsp *srvRspCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	return nil
}

// Serialize transforms the srvRsp into string form for
// inclusion through a file
func (srvRsp *srvRspCfg) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*srvRsp)
	} else {
		bytes, merr = json.Marshal(*srvRsp)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

func (srvRsp *srvRspCfg) CfgStr() string {
	rtn, err := srvRsp.Serialize(true)
	if err != nil {
		panic(fmt.Errorf("service response cfg serialization error"))
	}
	return rtn
}

// Deserialize recovers a serialized representation of a srvRsp structure
func (srvRsp *srvRspCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	example := srvRspCfg{Trace: 0}

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

// srvRspEnter flags
func srvRspEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {
	arpc := cpfi.Cfg.(*srvRspCfg)
	arps := cpfi.State.(*srvRspState)
	arps.calls += 1

	endptName := cpfi.Host
	endpt := mrnes.EndptDevByName[endptName]
	AddCPTrace(traceMgr, evtMgr.CurrentTime(), msg.ExecID, endpt.DevID(), "enter", msg)

	// look up response delay.  If message type prefix matches the direct prefix list
	// use it directly
	pieces := strings.Split(msg.MsgType, "-")
	var tcCode string
	if len(pieces) > 1 {
		found := false
		opPrefix := pieces[0]
		for _, prefix := range arpc.DirectPrefix {
			if opPrefix == prefix {
				found = true
				break
			}
		}
		if !found {
			panic(fmt.Errorf("expected server to have prefix %s for msg type %s", pieces[0], msg.MsgType))
		}
		tcCode = msg.MsgType
	} else {
		tcCode = arpc.TimingCode[msg.MsgType]
	}
	genTime := HostFuncExecTime(cpfi, tcCode, msg)

	msg.CPID = msg.RtnCPID
	msg.Label = msg.RtnLabel
	msg.MsgType = msg.RtnMsgType

	// put where ExitFunc will find it
	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})

	// schedule ExitFunc to happen after the genTime delay
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(genTime))
}

// -------- methods and state for function class srvReq
var srvReqVar *srvReqCfg = ClassCreateSrvReqCfg()
var srvReqLoaded bool = RegisterFuncClass(srvReqVar)

type srvReqState struct {
	rspEdgeIdx int     // index of edge pointing to service response function
	msgTypeIn  string  // type of message on receipt
	calls      int
}

type srvReqCfg struct {
	// map the service request to the message type on departure
	Bypass int               `yaml:"bypass" json:"bypass"`
	SrvCP  string            `yaml:"srvcp" json:"srvcp"`
	SrvOp  string            `yaml:"srvop" json:"srvop"`
	RspOp  string            `yaml:"rspop" json:"rspop"`
	SrvLabel string          `yaml:"srvlabel" json:"srvlabel"`
	Msg2MC map[string]string `yaml:"msg2mc" json:"msg2mc"`
	Msg2Msg map[string]string `yaml:"op2msg" json:"op2msg"`	
	Trace  int                `yaml:"trace" json:"trace"`
}

func ClassCreateSrvReqCfg() *srvReqCfg {
	srvReq := new(srvReqCfg)
	srvReq.Msg2MC = make(map[string]string)
	srvReq.Msg2Msg = make(map[string]string)
	return srvReq
}

func createSrvReqState() *srvReqState {
	srqs := new(srvReqState)
	srqs.rspEdgeIdx = -1 // flag for replacement on first encounter
	return srqs
}

func (srvReq *srvReqCfg) FuncClassName() string {
	return "srvReq"
}

func (srvReq *srvReqCfg) CreateCfg(cfgStr string) any {
	useYAML := (cfgStr[0] != '{')
	srvReqVarAny, err := srvReq.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("srvReq.InitCfg sees deserialization error"))
	}
	return srvReqVarAny
}

func (srvReq *srvReqCfg) Populate(bypass bool, trace bool) {
	if trace {
		srvReq.Trace = 1
	} else {
		srvReq.Trace = 0
	}
}

func (srvReq *srvReqCfg) InitCfg(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	srvReqVarAny := srvReq.CreateCfg(cfgStr)
	srvReqv := srvReqVarAny.(*srvReqCfg)
	cpfi.Cfg = srvReqv
	copyDict(cpfi.Msg2MC, srvReq.Msg2MC)
	cpfi.State = createSrvReqState()
	cpfi.Trace = (srvReqv.Trace != 0)
}

func (srvReq *srvReqCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	return nil
}

// Serialize transforms the srvReq into string form for
// inclusion through a file
func (srvReq *srvReqCfg) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*srvReq)
	} else {
		bytes, merr = json.Marshal(*srvReq)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

func (srvReq *srvReqCfg) CfgStr() string {
	rtn, err := srvReq.Serialize(true)
	if err != nil {
		panic(fmt.Errorf("service request cfg serialization error"))
	}
	return rtn
}

// Deserialize recovers a serialized representation of a srvReq structure
func (srvReq *srvReqCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	example := srvReqCfg{Trace: 0}

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

// srvReqEnter flags
func srvReqEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {
	srqs := cpfi.State.(*srvReqState)
	srqc := cpfi.Cfg.(*srvReqCfg)
	srqs.calls += 1

	if srqc.Bypass != 0 {
		// the outbound message type is the same as the inbound.
		// Find the outbound edge that matches

		outMsg := advanceMsg(cpfi, msg, msg.MsgType)		

		// put msg where ExitFunc will find it
		cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{outMsg})

		// schedule ExitFunc to happen after the genTime delay
		evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
		return
	}

	// remember incoming message type
	srqs.msgTypeIn = msg.MsgType

	var found bool
	if srqs.rspEdgeIdx == -1 {

		// find edge whose target func is from the "srvRsp" class
		for idx, edge := range cpfi.OutEdges {
			cp := CmpPtnInstByID[edge.CPID]
			for _, funcInst := range cp.Funcs {
				if funcInst.Class == "srvRsp" {
					srqs.rspEdgeIdx = idx
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}

	if found {
		var eidx = srqs.rspEdgeIdx
		edge := cpfi.OutEdges[eidx]
		msg.CPID = edge.CPID
		msg.Label = edge.FuncLabel
	} else {
		var srvCPID int
		var srvLabel string
		// the server CP needs to be given
		if len(srqc.SrvCP) > 0 {
			// srqc.SrvCP points to the CP and srvc.SrvOp gives the index in the Services table
			srvCPID = CmpPtnInstByName[srqc.SrvCP].ID
		} else {
			// for the use case of the first function on a processor asking for an authentication
			// service from the CmpPtn that called it
			srvCPID = CmpPtnInstByID[msg.PrevCPID].ID
		}

		// the server label is either given, or is looked up from the server CP services table
		if len(srqc.SrvLabel) > 0 {
			srvLabel = srqc.SrvLabel
		} else {
			// use the service op prefix as a code to look in the services table
			pieces := strings.Split(srqc.SrvOp, "-")
			srvOp := pieces[0]
		
			srvDesc := CmpPtnInstByID[srvCPID].Services[srvOp]
			if len(srvDesc.CP) > 0 {
				srvCPID = CmpPtnInstByName[srvDesc.CP].ID
			}
			srvLabel = srvDesc.Label
		}
		msg.CPID  = srvCPID
		msg.Label = srvLabel
		msg.MsgType = srqc.SrvOp
		msg.RtnCPID = cpfi.CPID
		msg.RtnLabel = cpfi.Label
		msg.RtnMsgType = "return-" + msg.MsgType

		// set up the return msgType to point to the return function
		cpfi.Msg2MC[msg.RtnMsgType] = "return"
	}

	// put msg where ExitFunc will find it
	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})

	endptName := cpfi.Host
	endpt := mrnes.EndptDevByName[endptName]
	AddCPTrace(traceMgr, evtMgr.CurrentTime(), msg.ExecID, endpt.DevID(), "enter", msg)

	// schedule ExitFunc to happen after the genTime delay
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
}

func srvReqRtn(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {
	srqs := cpfi.State.(*srvReqState)
	srqc := cpfi.Cfg.(*srvReqCfg)
	srqs.calls += 1

	endptName := cpfi.Host
	endpt := mrnes.EndptDevByName[endptName]
	AddCPTrace(traceMgr, evtMgr.CurrentTime(), msg.ExecID, endpt.DevID(), "enter", msg)

	// add response return processing delay, if indicated
	var rspTime	float64
	if len(srqc.RspOp) > 0 {
		rspTime = HostFuncExecTime(cpfi, srqc.RspOp, msg)
	}

	outMsgType := srqc.Msg2Msg[srqs.msgTypeIn]
	advanceMsg(cpfi, msg, outMsgType) 

	// schedule ExitFunc to happen after the delay of responding to service request
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(rspTime))
}

// -------- methods and state for function class transfer, to move
// a message from one comp pattern to another
var transferVar *transferCfg = ClassCreateTransferCfg()
var transferLoaded bool = RegisterFuncClass(transferVar)

type transferState struct {
	calls int
}

type transferCfg struct {
	Carried int               `yaml:"carried" json:"carried"` // message carries xCPID, xLabel
	XCP     string            `yaml:"xcp" json:"xcp"`         // CmpPtn name of destination
	XLabel  string            `yaml:"xlabel" json:"xlabel"`   // function label at destination
	Msg2MC  map[string]string `yaml:"msg2mc" json:"msg2mc"`
	Trace   int               `yaml:"trace" json:"trace"`
}

func ClassCreateTransferCfg() *transferCfg {
	trnsfr := new(transferCfg)
	return trnsfr
}

func createTransferState() *transferState {
	transfer := new(transferState)
	return transfer
}

func (trnsfr *transferCfg) FuncClassName() string {
	return "transfer"
}

func (trnsfr *transferCfg) CreateCfg(cfgStr string) any {
	useYAML := (cfgStr[0] != '{')
	transferVarAny, err := trnsfr.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("transfer.InitCfg sees deserialization error"))
	}
	return transferVarAny
}

func (trnsfr *transferCfg) Populate(carried bool, xcp string, xlabel string, trace bool) {
	if carried {
		trnsfr.Carried = 1
	} else {
		trnsfr.Carried = 0
	}
	trnsfr.XCP = xcp
	trnsfr.XLabel = xlabel
	if trace {
		trnsfr.Trace = 1
	} else {
		trnsfr.Trace = 1
	}
}

func (trnsfr *transferCfg) InitCfg(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	transferVarAny := trnsfr.CreateCfg(cfgStr)
	transferv := transferVarAny.(*transferCfg)
	cpfi.Cfg = transferv
	copyDict(cpfi.Msg2MC, transferv.Msg2MC)
	cpfi.State = createSrvReqState()
	cpfi.Trace = (transferv.Trace != 0)
}

func (trnsfr *transferCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	return nil
}

// Serialize transforms the transfer into string form for
// inclusion through a file
func (trnsfr *transferCfg) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*trnsfr)
	} else {
		bytes, merr = json.Marshal(*trnsfr)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

func (trnsfr *transferCfg) CfgStr() string {
	rtn, err := trnsfr.Serialize(true)
	if err != nil {
		panic(fmt.Errorf("service request cfg serialization error"))
	}
	return rtn
}

// Deserialize recovers a serialized representation of a transfer structure
func (trnsfr *transferCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	example := transferCfg{Trace: 0}

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

// transferEnter flags
func transferEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {
	tfrs := cpfi.State.(*transferState)
	tfrc := cpfi.Cfg.(*transferCfg)
	tfrs.calls += 1

	// the location of the CPID and Label depend on whether the function Carried bit is set
	if tfrc.Carried != 0 {
		// get ID of comp pattern where service to be requested resided
		msg.CPID = msg.XCPID
		msg.Label = msg.XLabel
	} else {
		msg.CPID = CmpPtnInstByName[tfrc.XCP].ID
		msg.Label = tfrc.XLabel
	}

	// put where ExitFunc will find it
	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})

	endptName := cpfi.Host
	endpt := mrnes.EndptDevByName[endptName]
	AddCPTrace(traceMgr, evtMgr.CurrentTime(), msg.ExecID, endpt.DevID(), "enter", msg)

	// schedule ExitFunc to happen immediately
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
}

//-------- methods and state for function class bckgrndLd

var bgLdVar *bckgrndLdCfg = ClassCreateBckgrndLdCfg()
var bckgrndLdLoaded bool = RegisterFuncClass(bgLdVar)

type bckgrndLdState struct {
	calls int
}

type bckgrndLdCfg struct {
	BckgrndFunc string  `yaml:"bckgrndfunc" json:"bckgrndfunc"`
	BckgrndRate float64 `yaml:"bckgrndrate" json:"bckgrndrate"`
	BckgrndSrv  float64 `yaml:"bckgrndsrv" json:"bckgrndsrv"`
	Trace       int     `yaml:"trace" json:"trace"`
}

func ClassCreateBckgrndLdCfg() *bckgrndLdCfg {
	bgld := new(bckgrndLdCfg)
	bgld.BckgrndFunc = "empty"
	bgld.BckgrndRate = 0.0
	bgld.BckgrndSrv = 0.010
	return bgld
}

func createBckgrndLdState() *bckgrndLdState {
	bglds := new(bckgrndLdState)
	return bglds
}

func (bgld *bckgrndLdCfg) FuncClassName() string {
	return "bckgrndLd"
}

func (bgld *bckgrndLdCfg) CreateCfg(cfgStr string) any {
	useYAML := (cfgStr[0] != '{')
	bgldVarAny, err := bgld.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("bckgrndLd.InitCfg sees deserialization error"))
	}
	return bgldVarAny
}

func (bgld *bckgrndLdCfg) InitCfg(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	bgldVarAny := bgld.CreateCfg(cfgStr)
	bgldv := bgldVarAny.(*bckgrndLdCfg)

	// default background event handler
	AddHndlrMap("genbckgrnd", generateBckgrndLd)

	// take the background load update function name and
	// use it to set up a function call
	_, present := HndlrMap[bgldv.BckgrndFunc]
	if !present {
		panic(fmt.Errorf("hndlr name %s unrecognized for bckgrndLd", bgldv.BckgrndFunc))
	}

	// schedule the initation of the background task
	evtMgr.Schedule(cpfi, nil, HndlrMap[bgldv.BckgrndFunc], vrtime.SecondsToTime(0.0))
	cpfi.Cfg = bgldv

	cpfi.State = createBckgrndLdState()
	cpfi.Trace = (bgldv.Trace != 0)

	// use the background parameters to impact the underlaying endpoint
	// background parameters
	/* save the below for the startup event handler
	endpt := mrnes.EndptDevByName[cpfi.Host]
	endpt.EndptState.BckgrndRate += bgldv.BckgrndRate
	endpt.EndptState.BckgrndSrv = math.Max(bgldv.BckgrndSrv, endpt.EndptState.BckgrndSrv)
	*/
}

func (bgld *bckgrndLdCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	return nil
}

// Serialize transforms the bckgrndLd into string form for
// inclusion through a file
func (bgld *bckgrndLdCfg) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*bgld)
	} else {
		bytes, merr = json.Marshal(*bgld)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a serialized representation of a bckgrndLd structure
func (bgld *bckgrndLdCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	example := bckgrndLdCfg{Trace: 0, BckgrndRate: 0.0, BckgrndSrv: 0.010}

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

// bckgrndGenLd will start the background load activity, but not schedule any changes
// to it
func generateBckgrndLd(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	cfg := cpfi.Cfg.(*bckgrndLdCfg)
	endpt := mrnes.EndptDevByName[cpfi.Host]
	endpt.EndptState.BckgrndRate += cfg.BckgrndRate
	endpt.EndptState.BckgrndSrv = math.Max(cfg.BckgrndSrv, endpt.EndptState.BckgrndSrv)
	mrnes.InitializeBckgrndEndpt(evtMgr, endpt)
	return nil
}

//-------- methods and state for function class measure

var measureVar *MeasureCfg = ClassCreateMeasureCfg()
var measureLoaded bool = RegisterFuncClass(measureVar)

type measureState struct {
	calls int
}

type MeasureCfg struct {
	MsrName string            `yaml:"msrname" json:"msrname"`
	MsrOp   string            `yaml:"msrop" json:"msrop"`
	Msg2MC  map[string]string `yaml:"msg2mc" json:"msg2mc"`
	Trace   int               `yaml:"trace" json:"trace"`
}

func ClassCreateMeasureCfg() *MeasureCfg {
	measure := new(MeasureCfg)
	return measure
}

func (measure *MeasureCfg) Populate(name string, pcktlen int, msglen int, start bool, op string, trace bool) {

	measure.MsrName = name
	if trace {
		measure.Trace = 1
	} else {
		measure.Trace = 0
	}
	measure.MsrOp = op
}

func createMeasureState() *measureState {
	measure := new(measureState)
	return measure
}

func (measure *MeasureCfg) FuncClassName() string {
	return "measure"
}

func (measure *MeasureCfg) CreateCfg(cfgStr string) any {
	useYAML := (cfgStr[0] != '{')
	measureVarAny, err := measure.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("measure.InitCfg sees deserialization error"))
	}
	return measureVarAny
}

func (measure *MeasureCfg) InitCfg(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	measureVarAny := measure.CreateCfg(cfgStr)
	measurev := measureVarAny.(*MeasureCfg)
	cpfi.Cfg = measurev
	copyDict(cpfi.Msg2MC, measurev.Msg2MC)
	cpfi.State = createMeasureState()
	cpfi.Trace = (measurev.Trace != 0)
}

func (measure *MeasureCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	return nil
}

// Serialize transforms the measure into string form for
// inclusion through a file
func (measure *MeasureCfg) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*measure)
	} else {
		bytes, merr = json.Marshal(*measure)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

func (measure *MeasureCfg) CfgStr() string {
	rtn, err := measure.Serialize(true)
	if err != nil {
		panic(fmt.Errorf("measure cfg serialization error"))
	}
	return rtn
}

// Deserialize recovers a serialized representation of a measure structure
func (measure *MeasureCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	prb := []byte(fss)

	example := MeasureCfg{Trace: 0}

	// Select whether we read in json or yaml
	if useYAML {
		err = yaml.Unmarshal(prb, &example)
	} else {
		err = json.Unmarshal(prb, &example)
	}

	if err != nil {
		return nil, err
	}
	return &example, nil
}

// measureEnter either starts or stops a measurement
func measureEnter(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {
	mcfg := cpfi.Cfg.(*MeasureCfg)

	endptName := cpfi.Host
	endpt := mrnes.EndptDevByName[endptName]
	AddCPTrace(traceMgr, evtMgr.CurrentTime(), msg.ExecID, endpt.DevID(), "exit", msg)

	// create an initial message
	if mcfg.MsrOp == "start" || mcfg.MsrOp == "Start" {
		if Measured == nil {
			Measured = new(MsrData)
			Measured.Measurements = make([]PerfRecord, 0)
		}
		createMeasureRecord(cpfi, mcfg.MsrName, evtMgr.CurrentSeconds(), msg)

		// should be only one edge
		edge := cpfi.OutEdges[0]
		msg.MsgType = edge.MsgType

		// put where ExitFunc will find it
		cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})
	} else if (mcfg.MsrName == measureID2Name[msg.MeasureID]) && (mcfg.MsrOp != "end" && mcfg.MsrOp != "End") {
		// this measurement has the same name as one that was tagged to the message
		cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})
		recordMeasure(msg.MeasureID, evtMgr.CurrentSeconds(), cpfi.Host, CmpPtnInstByID[cpfi.CPID].Name, cpfi.Label)
	} else if mcfg.MsrOp == "end" || mcfg.MsrOp == "End" {
		endMeasure(msg.MeasureID, evtMgr.CurrentSeconds(), cpfi.Host, CmpPtnInstByID[cpfi.CPID].Name, cpfi.Label)
	}

	// empty message type flags assertion that there is only one edge, so select it
	cpm := advanceMsg(cpfi, msg, "")
	evtMgr.Schedule(cpfi, cpm, ExitFunc, vrtime.SecondsToTime(0.0))
}
