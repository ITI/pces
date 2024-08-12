package pces

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
	"reflect"
	"sort"
)

// A FuncClass represents the methods used to simulate the effect
// of executing a function,  different types of input generate different
// types of responses, so we use a map whose key selects the start, end pair of methods
type FuncClassCfg interface {
	FuncClassName() string
	CreateCfg(string, bool) any
	InitCfg(*CmpPtnFuncInst, string, bool)
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
// might be included in a model.
var FuncClassNames map[string]bool = map[string]bool{"connSrc": true, "processPckt": true, "cycleDst": true, "finish": true}

// RegisterFuncClass is called to tell the system that a particular
// function class exists, and gives a point to its description.
// The idea is to register only those function classes actually used, or at least,
// provide clear separation between classes.  Reflect of bool allows call to RegisterFuncClass
// as part of a variable assignment outside of a function body
func RegisterFuncClass(fc FuncClassCfg) bool {
	className := fc.FuncClassName()
	_, present := FuncClasses[className]
	if present {
		panic(fmt.Errorf("attempt to register function class %s after that name already registered", fc.FuncClassName()))
	}
	FuncClasses[className] = fc
	FuncClassNames[className] = true
	return true
}

// FuncClasses is a map that takes us from the name of a FuncClass
var FuncClasses map[string]FuncClassCfg = make(map[string]FuncClassCfg)

// ClassMethods maps the function class name to a map indexed by (string) operation code to get to
// a pair of functions to handle the entry and exit to the function
var ClassMethods map[string]map[string]RespMethod = make(map[string]map[string]RespMethod)
var ClassMethodsBuilt bool = CreateClassMethods()

func CreateClassMethods() bool {
	// build table for connSrc class
	fmap := make(map[string]RespMethod)
	fmap["generateOp"] = RespMethod{Start: connSrcEnterStart, End: connSrcExitStart}
	fmap["completeOp"] = RespMethod{Start: connSrcEnterReflect, End: connSrcExitReflect}
	ClassMethods["connSrc"] = fmap

	fmap = make(map[string]RespMethod)
	fmap["generateOp"] = RespMethod{Start: cycleDstEnterStart, End: cycleDstExitStart}
	fmap["completeOp"] = RespMethod{Start: cycleDstEnterReflect, End: cycleDstExitReflect}
	ClassMethods["cycleDst"] = fmap

	// build table for processPckt class
	fmap = make(map[string]RespMethod)
	fmap["processOp"] = RespMethod{Start: processPcktEnter, End: processPcktExit}
	ClassMethods["processPckt"] = fmap

	// build table for finish class
	fmap = make(map[string]RespMethod)
	fmap["finishOp"] = RespMethod{Start: finishEnter, End: ExitFunc}
	ClassMethods["finish"] = fmap

	return true
}

// UpdateMsg copies the pattern, label coordinates of the message's current
// position to be the previous one and labels the next coordinates with
// the values given as arguments
func UpdateMsg(msg *CmpPtnMsg, nxtCPID int, nxtLabel, msgType, nxtMC string) {
	msg.MsgType = msgType
	msg.PrevCPID = msg.NxtCPID
	msg.NxtCPID =  nxtCPID
	msg.NxtMC = nxtMC
	msg.PrevLabel = msg.NxtLabel
	msg.NxtLabel = nxtLabel	
}

func validFuncClass(class string) bool {
	_, present := FuncClasses[class]
	return present
}

// advanceMsg acquires the method code of a just-completed operation, looks up the output edge
// associated with that method, and updates the message coordinates to
// direct it to the nxt location
func advanceMsg(cpfi *CmpPtnFuncInst, task *mrnes.Task, omi map[string]int, nxtMC string) *CmpPtnMsg {
	msg  := task.Msg.(*CmpPtnMsg)
	mc   := task.OpType

	eeidx := omi[mc]

	msgType := cpfi.OutEdges[eeidx].MsgType
	nxtCP   := cpfi.OutEdges[eeidx].CPID	
	nxtLabel := cpfi.OutEdges[eeidx].FuncLabel
	UpdateMsg(msg, nxtCP, nxtLabel, msgType, nxtMC)

	// put the response where ExitFunc will find it
	cpfi.AddResponse(msg.ExecID, []*CmpPtnMsg{msg})
	return msg
}	


//-------- methods and state for function class connSrc

var csrcVar *connSrcCfg = ClassCreateConnSrcCfg()
var connSrcLoaded bool = RegisterFuncClass(csrcVar)

type connSrcCfg struct {
	InterarrivalDist        string	`yaml:interarrivaldist json:interarrivaldist`
	InterarrivalMean        float64	`yaml:interarrivalmean json:interarrivalmean`
	InitMsgType             string	`yaml:initmsgtype json:initmsgtype`
	InitMsgLen				int		`yaml:initmsglen json:initmsglen`
	InitPcktLen				int		`yaml:initpcktlen json:initpcktlen`
	InitiationLimit         int		`yaml:initiationlimit json:initiationlimit`
	Route					map[string]string `yaml:route json:route`
	TimingCode				map[string]string `yaml:timingcode json:timingcode`
	Trace                   bool	`yaml:trace json:trace`
}

type connSrcState struct {
	calls		int
	returns		int
	outMsgIdx	map[string]int
}

func createConnSrcState() *connSrcState {
	cs := new(connSrcState)
	cs.calls = 0
	cs.returns = 0
	cs.outMsgIdx = make(map[string]int)
	return cs
}


func (cs *connSrcCfg) Populate(mean float64, limit int, interarrival, msgType string, 
		msgLen, pcktLen int, rt map[string]string, tc map[string]string, trace bool) {
	cs.InitMsgLen = msgLen
	cs.InitPcktLen = pcktLen
	cs.InitMsgType = msgType
	cs.InterarrivalDist = interarrival
	cs.InterarrivalMean = mean
	cs.InitiationLimit = limit
	cs.Route = rt
	cs.TimingCode = tc
	cs.Trace = trace
}

func ClassCreateConnSrcCfg() *connSrcCfg {
	cs := new(connSrcCfg)
	cs.InitMsgType = ""
	cs.InitMsgLen = 0
	cs.InitPcktLen = 0

	// Interarrival{Dist,Mean} defaults
	cs.InterarrivalDist = "const"
	cs.InterarrivalMean = 0.0
	cs.InitiationLimit = 0
	cs.Route = make(map[string]string)
	cs.TimingCode = make(map[string]string)
	cs.Trace = false
	return cs
}

func (cs *connSrcCfg) FuncClassName() string {
	return "connSrc"
}

func (cs *connSrcCfg) CreateCfg(cfgStr string, useYAML bool) any {
	csVarAny, err := cs.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("connSrc.InitCfg sees deserialization error"))
	}
	return csVarAny
}

// InitCfg saves the state encoded for the named cpfi, and copies
// the values given there for Interarrival{Dist, Mean} to the cpfi structure
func (cs *connSrcCfg) InitCfg(cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	csVarAny := cs.CreateCfg(cfgStr, useYAML)
	csv := csVarAny.(*connSrcCfg)
	cpfi.Cfg = csv

	cpfi.State = createConnSrcState()

	cpfi.Trace = csv.Trace
	cpfi.InitFunc = connSrcSchedule
	cpfi.InitMsgParams(csv.InitMsgType, csv.InitMsgLen, csv.InitPcktLen, 0.0)
	cpfi.InterarrivalDist = csv.InterarrivalDist
	cpfi.InterarrivalMean = csv.InterarrivalMean

}

func (csc *connSrcCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	state := cpfi.State.(*connSrcState)
	cfg := cpfi.Cfg.(*connSrcCfg)
	state.outMsgIdx = validateMCDicts(cpfi, cfg.Route, cfg.TimingCode)
	return nil
}
	
// connSrcSchedule schedules the next initiation at the comp pattern function instance pointed to
// by the context, and samples the next arrival time
func connSrcSchedule(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	csc := cpfi.Cfg.(*connSrcCfg)
	css := cpfi.State.(*connSrcState)
	cpi := CmpPtnInstByName[cpfi.PtnName]

	// if we do initiate messages from the implicitly named cpfi, see whether
	// we have done already all the ones programmed to be launched
	if csc.InitiationLimit > 0 && csc.InitiationLimit <= css.calls {
		return nil
	}

	// default interarrival time, if one is not otherwise chosen
	interarrival := float64(0.0)

	// introduce interarrival delay if an interarrival distribution is named
	if csc.InterarrivalDist != "none" {
		interarrival = csinterarrivalSample(cpi, csc.InterarrivalDist, csc.InterarrivalMean)
		evtMgr.Schedule(cpfi, nil, EnterFunc, vrtime.SecondsToTime(interarrival))
	} else {
		evtMgr.Schedule(cpfi, nil, EnterFunc, vrtime.CreateTime(1, 0))
	}

	// the absence of a message flags (along with cpfi.InitFunc not being empty)
	// tells EnterFunc to spawn a new message

	// skip interarrival generation of source generation if no interarrival distribution named.  Means it is generated elsewhere,
	// probably based on completion of some task
	if csc.InterarrivalDist != "none" {
		evtMgr.Schedule(cpfi, nil, connSrcSchedule, vrtime.SecondsToTime(interarrival))
	}
	return nil
}

// serialize transforms the connSrc into string form for
// inclusion through a file
func (csc *connSrcCfg) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*csc)
	} else {
		bytes, merr = json.Marshal(*csc)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a serialized representation of a connSrc structure
func (csc *connSrcCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	rt := make(map[string]string)
	tc := make(map[string]string)

	example := connSrcCfg{Route: rt, TimingCode: tc}

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

	csc := cpfi.Cfg.(*connSrcCfg)
	css := cpfi.State.(*connSrcState)
	css.calls += 1

	// look up the generation service requirement
	genTime := FuncExecTime(cpfi, csc.TimingCode[methodCode], msg)

	// call the host's scheduler.
	mrnes.TaskSchedulerByHostName[cpfi.Host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, connSrcExitStart)
}

// connSrcExitStart executes at the time when the initiating connSrc activation completes
func connSrcExitStart(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	css := cpfi.State.(*connSrcState)
	task := data.(*mrnes.Task)

	msg := advanceMsg(cpfi, task, css.outMsgIdx, "")

	// schedule the exitFunc handler
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

// connSrcEnterReflect deals with the arrival of a return from the original message
func connSrcEnterReflect(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {

	csc := cpfi.Cfg.(*connSrcCfg)

	// look up the generation service requirement
	genTime := FuncExecTime(cpfi, csc.TimingCode[methodCode], msg)

	// N.B. perhaps we can schedule ExitFunc here rather than
	mrnes.TaskSchedulerByHostName[cpfi.Host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, connSrcExitReflect)
}

// connSrcEnd executes at the completion of the return processing delay.
// Do we need to make the output message slice available here?
func connSrcExitReflect(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	css := cpfi.State.(*connSrcState)
	task := data.(*mrnes.Task)

	msg := advanceMsg(cpfi, task, css.outMsgIdx, "")

	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

//-------- methods and state for function class cycleDst

var cdstVar *cycleDstCfg = ClassCreateCycleDstCfg()
var cycleDstLoaded bool = RegisterFuncClass(cdstVar)

type cycleDstCfg struct {
	PcktDist  string	`yaml:pcktdist json:pcktdist`
	PcktMu    float64	`yaml:pcktmu json:pcktmu`
	BurstDist string	`yaml:burstdist json:burstdist`
	BurstMu   float64	`yaml:burstmu json:burstmu`
	BurstLen  int		`yaml:burstlen json:burstlen`
	CycleDist string	`yaml:cycledist json:cycledist`
	CycleMu   float64	`yaml:cyclemu json:cyclemu`
	Cycles int			`yaml:cycles json:cycles`
	InitMsgType string	`yaml:initmsgtype json:initmsgtype`
	InitMsgLen int		`yaml:initmsglen json:initmsglen`
	InitPcktLen int		`yaml:initpcktlen json:initpcktlen`
	Dsts []string		`yaml:dsts json:dsts`
	Trace   bool		`yaml:trace json:trace`
	Route	map[string]string `yaml:route json:route`
	TimingCode map[string]string `yaml:timingcode json:timingcode`
}

type cycleDstState struct {
	cycleID int
	burstID int
	dstID   int
	pckts	int
	calls   int
	returns int
	outMsgIdx  map[string]int
}

func createCycleDstState() *cycleDstState {
	cds := new(cycleDstState)
	cds.outMsgIdx = make(map[string]int)
	return cds
}


func (cd *cycleDstCfg) Populate(dsts []string, 
	pcktDist string, pcktMu float64, burstDist string, burstMu float64, pcktBurst int,
	cycleDist string, cycleMu float64, cycles int,
	msgLen, pcktSize int, 
	rt map[string]string, tc map[string]string, trace bool) {

	cd.PcktDist = pcktDist
	cd.PcktMu = pcktMu

	cd.BurstDist = burstDist
	cd.BurstMu = burstMu
	cd.BurstLen = pcktBurst

	cd.CycleDist = cycleDist
	cd.CycleMu = cycleMu
	cd.Cycles = cycles

	cd.InitMsgLen = msgLen
	cd.InitPcktLen = pcktSize
	cd.InitMsgType = "initiate"

	cd.Dsts = make([]string,len(dsts))
	for idx, dst := range dsts {
		cd.Dsts[idx] = dst
	}
	cd.Trace = trace
	cd.Route = rt
	cd.TimingCode = tc
}

func ClassCreateCycleDstCfg() *cycleDstCfg {
	cd := new(cycleDstCfg)
	cd.Route = make(map[string]string)
	cd.TimingCode = make(map[string]string)
	return cd
}

func (cd *cycleDstCfg) FuncClassName() string {
	return "cycleDst"
}

func (cdc *cycleDstCfg) CreateCfg(cfgStr string, useYAML bool) any {
	cdVarAny, err := cdc.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("cycleDst.InitCfg sees deserialization error"))
	}
	return cdVarAny
}

// InitCfg saves the state encoded for the named cpfi, and copies
// the values given there for Interarrival{Dist, Mean} to the cpfi structure
func (cdc *cycleDstCfg) InitCfg(cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	cdVarAny := cdc.CreateCfg(cfgStr, useYAML)
	cdv := cdVarAny.(*cycleDstCfg)
	cpfi.Cfg = cdv

	cpfi.State = createCycleDstState()

	cpfi.Trace = cdv.Trace
	cpfi.InitFunc = cycleDstSchedule

	cpfi.InitMsgParams(cdv.InitMsgType, cdv.InitMsgLen, cdv.InitPcktLen, 0.0)
}

func (cdc *cycleDstCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	state := cpfi.State.(*cycleDstState)
	cfg := cpfi.Cfg.(*cycleDstCfg)
	state.outMsgIdx = validateMCDicts(cpfi, cfg.Route, cfg.TimingCode)
	return nil
}

// scheduleBurst schedules the immediate generation of as many
// packts for the target EUD as are configured.
func scheduleBurst(evtMgr *evtm.EventManager, cpi *CmpPtnInst, cpfi *CmpPtnFuncInst, cpm *CmpPtnMsg, after float64) {
	cdc := cpfi.Cfg.(*cycleDstCfg)
	advance := after
	for idx := 0; idx < cdc.BurstLen; idx++ {
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
		evtMgr.Schedule(cpfi, newMsg, EnterFunc, vrtime.SecondsToTime(advance))

		if idx < cdc.BurstLen-1 {
			after = cdc.PcktMu
			if cdc.PcktDist == "exp" {
				after = csinterarrivalSample(cpi, "exp", cdc.PcktMu)
			}
			advance += after
		}
	}
}

// cycleDstSchedule schedules the next burst.
func cycleDstSchedule(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	cdc := cpfi.Cfg.(*cycleDstCfg)
	cds := cpfi.State.(*cycleDstState)
	cpi := CmpPtnInstByName[cpfi.PtnName]

	// default interarrival time for bursts, if one is not otherwise chosen
	interarrival := float64(0.0)

	pcktsPerCycle := cdc.Cycles*cdc.BurstLen
	if cds.pckts > 0 && cds.pckts%pcktsPerCycle == 0 {
		if cdc.CycleDist == "exp" {
			interarrival = csinterarrivalSample(cpi, cdc.CycleDist, cdc.CycleMu)
		} else {
			interarrival = cdc.CycleMu
		}
	} else {
		if cdc.BurstDist == "exp" {
			interarrival = csinterarrivalSample(cpi, cdc.BurstDist, cdc.BurstMu)
		} else {
			interarrival = cdc.BurstMu
		}
	}

	cpm := new(CmpPtnMsg)
	*cpm = *cpfi.InitMsg
	cpm.ExecID = numExecThreads
	cpm.MsgType = "initiate"
	numExecThreads += 1

	cpm.PrevCPID = cpi.ID
	cpm.PrevLabel = cpfi.Label

	// for initiation only
	cpm.NxtCPID = cpi.ID
	cpm.NxtLabel = cpfi.Label

	numDsts := len(cdc.Dsts)
	cpi = CmpPtnInstByName[ cdc.Dsts[ cds.burstID%numDsts ] ]
	cpm.CmpHdr.EndCPID = cpi.ID

	// note that we don't know the label of the function this will eventually go to,
	// but trust to the edges on the path the message traverses to guide it there.

	// launch the burst
	scheduleBurst(evtMgr, cpi, cpfi, cpm, interarrival)

	cds.burstID += 1
	if cds.burstID%numDsts == 0 {
		cds.cycleID += 1
	}

	// schedule the next burst schedule to occur at the same time (but after)
	// the burst just scheduled
	if cds.cycleID < cdc.Cycles {
		evtMgr.Schedule(cpfi, nil, cycleDstSchedule, vrtime.SecondsToTime(interarrival))
	}
	return nil
}

// serialize transforms the cycleDst into string form for
// inclusion through a file
func (cdc *cycleDstCfg) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*cdc)
	} else {
		bytes, merr = json.Marshal(*cdc)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// Deserialize recovers a serialized representation of a cycleDst structure
func (cdc *cycleDstCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	rt := make(map[string]string)
	tc := make(map[string]string)
	dsts := make([]string,0)

	example := cycleDstCfg{Route: rt, TimingCode:tc, Dsts: dsts }

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

	cdc := cpfi.Cfg.(*cycleDstCfg)
	cds := cpfi.State.(*cycleDstState)
	cds.calls += 1

	// look up the generation service requirement
	genTime := FuncExecTime(cpfi, cdc.TimingCode[methodCode], msg)

	// call the host's scheduler.
	mrnes.TaskSchedulerByHostName[cpfi.Host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, cycleDstExitStart)
}

// cycleDstExitStart executes at the time when the initiating cycleDst activation completes
func cycleDstExitStart(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	cds := cpfi.State.(*cycleDstState)
	task := data.(*mrnes.Task)

	msg := advanceMsg(cpfi, task, cds.outMsgIdx, "")

	// schedule the exitFunc handler
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}

// cycleDstEnterReflect deals with the arrival of a return from the original message
func cycleDstEnterReflect(evtMgr *evtm.EventManager, cpfi *CmpPtnFuncInst, methodCode string, msg *CmpPtnMsg) {

	cds := cpfi.State.(*cycleDstState)
	cdc := cpfi.Cfg.(*cycleDstCfg)
	cds.returns += 1

	// look up the generation service requirement
	genTime := FuncExecTime(cpfi, cdc.TimingCode[methodCode], msg)

	// call the host's scheduler.
	mrnes.TaskSchedulerByHostName[cpfi.Host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, cycleDstExitReflect)
}

// cycleDstEnd executes at the completion of packet generation
func cycleDstExitReflect(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	cds := cpfi.State.(*cycleDstState)
	task := data.(*mrnes.Task)

	msg := advanceMsg(cpfi, task, cds.outMsgIdx, "")

	cpfi.MsgResp[msg.ExecID] = []*CmpPtnMsg{msg}

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

func validateMCDicts(cpfi *CmpPtnFuncInst, rt, tc map[string]string) map[string]int {
	routeKeys := []string{}
	for key, _ := range rt {
		routeKeys = append(routeKeys, key)
	}
	sort.Strings(routeKeys)

	tcKeys := []string{}
	for key, _ := range tc {
		tcKeys = append(tcKeys, key)
	}
	sort.Strings(tcKeys)

	if !reflect.DeepEqual(routeKeys, tcKeys) {
		panic(fmt.Errorf("method code indexing problem in processPckt initialization for %s\n", cpfi.Label))
	}

	omi := make(map[string]int)

	// given ppv.EgressMsgType find the first outEdge in cpfi's list
	// that matches
	for _, mc := range routeKeys {
		found := 0
		emt := rt[mc] 

		for idx, edge := range cpfi.OutEdges {
			if emt == edge.MsgType {
				omi[mc] = idx
				found += 1
			}
		}
		if found == 0 {
			panic(fmt.Errorf("initialization of Func in %s cites non-existant egress edge", cpfi.Label))
		} else if found > 1 {
			panic(fmt.Errorf("initialization of Func in %s cites multiple egress edges for a method code", cpfi.Label))
		}
	}
	return omi
}


//-------- methods and state for function class processPckt

var ppVar *processPcktCfg = ClassCreateProcessPcktCfg()
var processPcktLoaded bool = RegisterFuncClass(ppVar)

type processPcktCfg struct {
	Route	  map[string]string   `yaml:route json:route`
	TgtCP     map[string]string   `yaml:tgtcp json:tgtcp`
	TgtLabel  map[string]string   `yaml:tgtlabel json:tgtlabel`
	TimingCode map[string]string  `yaml:timingcode json:timingcode`
	Trace     bool		`yaml:trace json:trace`
}

type processPcktState struct {
	calls     int				  
	outMsgIdx map[string]int	  
}

// ClassCreateProcessPckt is a constructor called just to create an instance, fields unimportant
func ClassCreateProcessPcktCfg() *processPcktCfg {
	pp := new(processPcktCfg)
	pp.Route = make(map[string]string)
	pp.TimingCode = make(map[string]string)
	pp.TgtCP    = make(map[string]string)
	pp.TgtLabel = make(map[string]string)

	pp.Trace = false
	return pp
}


func createProcessPcktState() *processPcktState {
	pps := new(processPcktState)
	pps.outMsgIdx = make(map[string]int)
	return pps
}

func (pp *processPcktCfg) FuncClassName() string {
	return "processPckt"
}

func (pp *processPcktCfg) CreateCfg(cfgStr string, useYAML bool) any {
	ppVarAny, err := pp.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("processPckt.InitCfg sees deserialization error"))
	}
	return ppVarAny
}

func (pp *processPcktCfg) InitCfg(cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	ppVarAny := pp.CreateCfg(cfgStr, useYAML)
	ppv := ppVarAny.(*processPcktCfg)
	cpfi.Cfg = ppv
	cpfi.State = createProcessPcktState()
	cpfi.Trace = ppv.Trace
}

func (pp *processPcktCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	state := cpfi.State.(*processPcktState)
	cfg := cpfi.Cfg.(*processPcktCfg)
	state.outMsgIdx = validateMCDicts(cpfi, cfg.Route, cfg.TimingCode)
	return nil
}

// serialize transforms the processPckt into string form for
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

// Deserialize recovers a serialized representation of a processPckt structure
func (pp *processPcktCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	rt    := make(map[string]string)
	tc := make(map[string]string)
	tgtcp := make(map[string]string)
	tgtlbl := make(map[string]string)
	
	example := processPcktCfg{Route: rt, TgtCP: tgtcp, TgtLabel: tgtlbl, TimingCode: tc, Trace: false}

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

	pps := cpfi.State.(*processPcktState)
	ppc := cpfi.Cfg.(*processPcktCfg)
	pps.calls += 1

	// look up the generation service requirement.
	genTime := FuncExecTime(cpfi, ppc.TimingCode[methodCode], msg)

	// call the host's scheduler.
	mrnes.TaskSchedulerByHostName[cpfi.Host].Schedule(evtMgr,
		methodCode, genTime, math.MaxFloat64, cpfi, msg, processPcktExit)
}

// processPcktExit executes when the associated message did not get served immediately on being scheduled,
// but now has finished.
func processPcktExit(evtMgr *evtm.EventManager, context any, data any) any {
	cpfi := context.(*CmpPtnFuncInst)
	task := data.(*mrnes.Task)
	pps := cpfi.State.(*processPcktState)
	ppg := cpfi.Cfg.(*processPcktCfg)

	msg := advanceMsg(cpfi, task, pps.outMsgIdx, "")

	// if the associated method code has an entry in the Func's globalDst dictionary
	// change the global destination CP and label
	mc := task.OpType
	_, present := ppg.TgtCP[mc]
	if present {
		tgtCP := ppg.TgtCP[mc]
		tgtCPID := CmpPtnInstByName[tgtCP].ID	
		msg.CmpHdr.EndCPID  = tgtCPID
	}
	_, present = ppg.TgtLabel[mc]
	if present {
		tgtLabel := ppg.TgtLabel[mc]
		msg.CmpHdr.EndLabel = tgtLabel
	}

	// schedule the exitFunc handler
	evtMgr.Schedule(cpfi, msg, ExitFunc, vrtime.SecondsToTime(0.0))
	return nil
}


//-------- methods and state for function class finish

var fnshVar *finishCfg = ClassCreateFinishCfg()
var finishLoaded bool = RegisterFuncClass(fnshVar)

type finishState struct {
	calls int
}

type finishCfg struct {
	Trace     bool		`yaml:trace json:trace`
}

func ClassCreateFinishCfg() *finishCfg {
	fnsh := new(finishCfg)
	fnsh.Trace = false
	return fnsh
}

func createFinishState() *finishState {
	fnsh := new(finishState)
	return fnsh
}	


func (fnsh *finishCfg) FuncClassName() string {
	return "finish"
}

func (fnsh *finishCfg) CreateCfg(cfgStr string, useYAML bool) any {
	fnshVarAny, err := fnsh.Deserialize(cfgStr, useYAML)
	if err != nil {
		panic(fmt.Errorf("finish.InitCfg sees deserialization error"))
	}
	return fnshVarAny
}

func (fnsh *finishCfg) InitCfg(cpfi *CmpPtnFuncInst, cfgStr string, useYAML bool) {
	fnshVarAny := fnsh.CreateCfg(cfgStr, useYAML)
	fnshv := fnshVarAny.(*finishCfg)
	cpfi.Cfg = fnshv
	cpfi.State = createFinishState()
	cpfi.Trace = fnshv.Trace
}

func (fnsh *finishCfg) ValidateCfg(cpfi *CmpPtnFuncInst) error {
	return nil
}


// serialize transforms the finish into string form for
// inclusion through a file
func (fnsh *finishCfg) Serialize(useYAML bool) (string, error) {
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
func (fnsh *finishCfg) Deserialize(fss string, useYAML bool) (any, error) {
	// turn the string into a slice of bytes
	var err error
	fsb := []byte(fss)

	example := finishCfg{Trace: false}

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

	fns := cpfi.State.(*finishState)
	fns.calls += 1

	endptName := cpfi.Host
	endpt := mrnes.EndptDevByName[endptName]
	traceMgr.AddTrace(evtMgr.CurrentTime(), msg.ExecID, 0, endpt.DevID(), "exit", msg.CarriesPckt(), msg.Rate)
	EndRecExec(msg.ExecID, evtMgr.CurrentSeconds())
}
