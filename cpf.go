package pces

// file cpf.go holds structs and methods related to instances of computational pattern functions

import (
	"fmt"
	"github.com/iti/evt/evtm"
)

// The edgeStruct struct describes a possible output edge for a response
type edgeStruct struct {
	CPID      int    // ID of comp pattern
	FuncLabel string // might be source or destination function label, depending on context
	MsgType   string
}

func createEdgeStruct(cpID int, label, msgType string) edgeStruct {
	es := edgeStruct{CPID: cpID, FuncLabel: label, MsgType: msgType}
	return es
}

// The CmpPtnFuncInst struct represents an instantiated instance of a function
type CmpPtnFuncInst struct {
	InitFunc         evtm.EventHandlerFunction // if not 'emptyInitFunc' call this to initialize the function
	AuxFunc          evtm.EventHandlerFunction  
	InitMsg          *CmpPtnMsg                // message that is copied when this instance is used to initiate a chain of func evaluations
	Class            string                    // specifier leading to specific state, entrance, and exit functions
	Label            string                    // an identifier for this func, unique within the instance of CompPattern holding it.
	Host             string                    // identity of the host to which this func is mapped for execution
	SharedGroup      string                    // empty means state not shared, otherwise global name of group with shared state
	PtnName          string                    // name of the instantiated CompPattern holding this function
	CPID             int                       // id of the comp pattern this func is attached to
	ID               int                       // integer identity which is unique among all objects in the pces model
	Active           bool                      // flag whether function is actively processing inputs
	Trace            bool                      // indicate whether this function should record its enter/exit in the trace
	Cfg				 any                       // holds string-coded state for string-code configuratin variable names
	State            any                       // holds string-coded state for string-code state variable names
	InterarrivalDist string
	InterarrivalMean float64

	// represent the comp pattern edges touching this function.  The in edge
	// is indexed by the source function label and message type, yielding the method code
	InEdgeMethodCode map[edgeStruct]string

	// the out edges are in a list of edgeStructs
	OutEdges []edgeStruct

	// respMethods indexed by method code
	RespMethods map[string]*RespMethod

	// save messages created as result of processing function
	MsgResp map[int][]*CmpPtnMsg

	// save ids of connections launched from this function
	SendConnectID map[int]bool

	// save ids of connections launched from this function
	RecvConnectID map[int]bool

}

// createDestFuncInst== is a constructor that builds an instance of CmpPtnFunctInst from a Func description and
// a serialized representation of a StaticParameters struct.
func createFuncInst(cpInstName string, cpID int, fnc *Func, cfgStr string, useYAML bool) *CmpPtnFuncInst {
	cpfi := new(CmpPtnFuncInst)
	cpfi.ID = nxtID()         // get an integer id that is unique across all objects in the simulation model
	cpfi.Label = fnc.Label    // remember a label given to this function instance as part of building a CompPattern graph
	cpfi.PtnName = cpInstName // remember the name of the instance of the comp pattern in which this func resides
	cpfi.CPID = cpID          // remember the ID of the Comp Pattern in which this func resides
	cpfi.InitFunc = nil       // will be over-ridden if there is an initialization event scheduled later
	cpfi.InitMsg = nil        // will be over-ridden if the initialization block indicates initiation possible
	cpfi.Active = true
	cpfi.Trace = false
	cpfi.Class = fnc.Class
	cpfi.MsgResp = make(map[int][]*CmpPtnMsg)

	// inEdges, outEdges, and methodCode filled in after all function instances for a comp pattern created
	cpfi.InEdgeMethodCode = make(map[edgeStruct]string)
	cpfi.OutEdges = make([]edgeStruct, 0)
	// cpfi.methodCode = make(map[string]string)
	cpfi.RespMethods = make(map[string]*RespMethod)

	// if the ClassMethods map for the cpfi's class exists,
	// initialize respMethods to be that

	_, present := ClassMethods[fnc.Class]
	if present {
		// copy over the mapping to event handlers
		for mc, mcr := range ClassMethods[fnc.Class] {
			mcrcpy := new(RespMethod)
			*mcrcpy = mcr
			cpfi.RespMethods[mc] = mcrcpy
		}
	}

	// look up the func-to-host assignment, established earlier in the initialization sequence
	cpfi.Host = CmpPtnFuncHost(cpfi)

	// get a pointer to the function's class
	fc := FuncClasses[fnc.Class]

	// if the function is not shared we initialize its state from the cfgStr string.
	// otherwise we have already created the state and just recover it
	// FINDME N.B. revisit notion/use of shared function
	if len(cpfi.SharedGroup) == 0 {
		fc.InitCfg(cpfi, cfgStr, useYAML)
	} else {
		gfid := GlobalFuncID{CmpPtnName: cpfi.PtnName, Label: cpfi.Label}
		cpfi.Cfg = funcInstToSharedCfg[gfid]
	}

	if cpfi.funcTrace() {
		traceMgr.AddName(cpfi.ID, cpfi.GlobalName(), "application")
	}

	cpfi.SendConnectID = make(map[int]bool)
	cpfi.RecvConnectID = make(map[int]bool)

	return cpfi
}

func (cpfi *CmpPtnFuncInst) GlobalName() string {
	return cpfi.PtnName + ":" + cpfi.Label
}

// AddResponse stores the selected out message response from executing the function,
// to be released later.  Saving through cpfi.Resp[execID] to account for concurrent overlapping executions
func (cpfi *CmpPtnFuncInst) AddResponse(execID int, resp []*CmpPtnMsg) {
	_, present := cpfi.MsgResp[execID]

	// add response for this function instance with this execId if it is not already present
	if !present {
		cpfi.MsgResp[execID] = resp
	}
}

// funcResp returns the saved list of function response messages associated
// the the response to the input msg, and removes it from the msgResp map
func (cpfi *CmpPtnFuncInst) funcResp(execID int) []*CmpPtnMsg {

	rtn, present := cpfi.MsgResp[execID]
	if !present {
		panic(fmt.Errorf("unsuccessful resp recovery"))
	}
	delete(cpfi.MsgResp, execID)

	return rtn
}

func (cpfi *CmpPtnFuncInst) isInitiating() bool {
	return cpfi.InitFunc != nil
}

func (cpfi *CmpPtnFuncInst) InitMsgParams(msgType string, msgLen, pcktLen int, rate float64) {
	if !(msgLen>0 && pcktLen>0) {
		panic(fmt.Errorf("zero lengths msg %d, pckt %d", msgLen, pcktLen))
	}

	cpfi.InitMsg = &CmpPtnMsg{MsgType: msgType, MsgLen: msgLen, PcktLen: pcktLen, Rate: rate, 
		MrnesConnDesc: DefaultConnDesc(), MrnesRprts: DefaultRprts(), MIN: NxtMIN()}
}

// funcActive indicates whether the function is processing messages
func (cpfi *CmpPtnFuncInst) funcActive() bool {
	return cpfi.Active
}

// funcClass gives the FuncClass declares on the Func upon which this instance is based
func (cpfi *CmpPtnFuncInst) funcClass() string {
	return cpfi.Class
}

// funcLabel gives the unique-within-CmpPtnInst label identifier for the function
func (cpfi *CmpPtnFuncInst) funcLabel() string {
	return cpfi.Label
}

// funcID returns the unique-across-model integer identifier for this CmpPtnFunc
func (cpfi *CmpPtnFuncInst) funcID() int {
	return cpfi.ID
}

// funcCmpPtn gives the name of the CmpPtnInst the CmpPtnFunc is part of
func (cpfi *CmpPtnFuncInst) funcCmpPtn() string {
	return cpfi.PtnName
}

// funcTrace returns the value of the trace attribute
func (cpfi *CmpPtnFuncInst) funcTrace() bool {
	return cpfi.Trace
}

// funcDevice returns the name of the topological Host upon which this CmpPtnFunc executes
func (cpfi *CmpPtnFuncInst) funcDevice() string {
	return cpfi.Host
}

// func funcInitEvtHdlr gives the function to schedule to initialize the function instance
func (cpfi *CmpPtnFuncInst) funcInitEvtHdlr() evtm.EventHandlerFunction {
	return cpfi.InitFunc
}

// AddStartMethod associates a string response 'name' with a pair of methods used to
// represent the start and end of the function's execution.   The default method
// for the end method is ExitFunc
// Return of bool allows call to RegisterFuncClass
// as part of a variable assignment outside of a function body
func (cpfi *CmpPtnFuncInst) AddStartMethod(methodCode string, start StartMethod) bool {
	// if the name for the methods is already in use, panic
	_, present := cpfi.RespMethods[methodCode]
	if present {
		panic(fmt.Errorf("function method name %s already in use", methodCode))
	}

	rp := RespMethod{Start: start, End: ExitFunc}
	cpfi.RespMethods[methodCode] = &rp
	return true
}

// AddEndMethod includes a customized End method to a previously defined respMethod
func (cpfi *CmpPtnFuncInst) AddEndMethod(methodCode string, end evtm.EventHandlerFunction) bool {
	_, present := cpfi.RespMethods[methodCode]
	if !present {
		panic(fmt.Errorf("AddEndMethod requires methodCode %s to have been declared already", methodCode))
	}
	cpfi.RespMethods[methodCode].End = end
	return true
}

func CmpPtnFuncHost(cpfi *CmpPtnFuncInst) string {
	ptnMap := CmpPtnMapDict.Map[cpfi.PtnName] // note binding of pattern to dictionary map to function map.  Shared needs something different
	return ptnMap.FuncMap[cpfi.Label]
}
