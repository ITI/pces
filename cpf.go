package pces

// file cpf.go holds structs and methods related to instances of computational pattern functions

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"strconv"
	"strings"
)

// The edgeStruct struct describes a possible output edge for a response
type edgeStruct struct {
	CPID      int    // ID of comp pattern
	FuncLabel string // might be source or destination function label, depending on context
	MsgType   string
}

// createEdgeStruct is a constructor
func createEdgeStruct(cpID int, label, msgType string) edgeStruct {
	es := edgeStruct{CPID: cpID, FuncLabel: label, MsgType: msgType}
	return es
}

// The CmpPtnFuncInst struct represents an instantiated instance of a function
type CmpPtnFuncInst struct {
	AuxFunc     evtm.EventHandlerFunction
	Class       string             // specifier leading to specific state, entrance, and exit functions
	Label       string             // an identifier for this func, unique within the instance of CompPattern holding it.
	Host        string             // identity of the host to which this func is mapped for execution
	SharedGroup string             // empty means state not shared, otherwise global name of group with shared state
	PtnName     string             // name of the instantiated CompPattern holding this function
	CPID        int                // id of the comp pattern this func is attached to
	ID          int                // integer identity which is unique among all objects in the pces model
	Trace       bool               // indicate whether this function should record its enter/exit in the trace
	Measure     bool               // If true start device-to-device measurement
	IsService   bool               // when a service the processing does not depend on the CP asking for a response
	PrevEdge    map[int]edgeStruct // saved selection edge
	Priority    int				   // scheduling priority
	Cfg         any                // holds string-coded state for string-code configuratin variable names
	State       any                // holds string-coded state for string-code state variable names

	// OutEdges is a list of edgeStructs
	OutEdges []edgeStruct

	// input message type to method code
	Msg2MC map[string]string

	// output message type to index in OutEdges
	Msg2Idx map[string]int

	// list of user-defined groups
	Groups []string

	// RespMethods are indexed by method code.
	// A Respond method holds a pointer to a function to call when processing
	// an input with the given response method, and a function to schedule
	// when the method has completed.
	RespMethods map[string]*RespMethod

	// MsgResp maps a thread's execID to a list of computation pattern messages
	MsgResp map[int][]*CmpPtnMsg
}

// createDestFuncInst is a constructor that builds an instance of CmpPtnFunctInst from a Func description and
// a serialized representation of a StaticParameters struct.
func createFuncInst(cpInstName string, cpID int, fnc *Func, cfgStr string, useYAML bool, evtMgr *evtm.EventManager) *CmpPtnFuncInst {
	cpfi := new(CmpPtnFuncInst)
	cpfi.ID = nxtID()         // get an integer id that is unique across all objects in the simulation model
	cpfi.Label = fnc.Label    // remember a label given to this function instance as part of building a CompPattern graph
	cpfi.PtnName = cpInstName // remember the name of the instance of the comp pattern in which this func resides
	cpfi.CPID = cpID          // remember the ID of the Comp Pattern in which this func resides
	cpfi.Trace = false        // flag whether we should trace execution through this function
	cpfi.IsService = false
	cpfi.Class = fnc.Class // remember the class
	cpfi.MsgResp = make(map[int][]*CmpPtnMsg) // prepare to be initialized

	// inEdges, outEdges, and methodCode filled in after all function instances for a comp pattern created
	cpfi.OutEdges = make([]edgeStruct, 0)
	cpfi.Msg2MC = make(map[string]string)
	cpfi.Msg2Idx = make(map[string]int)

	cpfi.RespMethods = make(map[string]*RespMethod) // prepare to be initialized

	cpfi.Groups = make([]string,0)

	// if the ClassMethods map for the cpfi's class exists,
	// initialize respMethods to be that

	// the ClassMethods map associates a dictionary with the name of the
	// function class.  That dictionary is indexed by 'methodCode' and has as
	// a value a RespMethod structure that describes
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
	cpfi.Priority = CmpPtnFuncPriority(cpfi)

	// get a pointer to the function's class
	fc := FuncClasses[fnc.Class]

	// if the function is not shared we initialize its state from the cfgStr string.
	// otherwise we have already created the state and just recover it
	// FINDME N.B. revisit notion/use of shared function
	if len(cpfi.SharedGroup) == 0 {
		fc.InitCfg(evtMgr, cpfi, cfgStr, useYAML)
		cpi := CmpPtnInstByID[cpfi.CPID]
		for _, grp := range cpfi.Groups {
			cpi.AddFuncToGroup(cpfi, grp)
		}
	} else {
		gfid := GlobalFuncID{CmpPtnName: cpfi.PtnName, Label: cpfi.Label}
		cpfi.Cfg = funcInstToSharedCfg[gfid]
	}

	if cpfi.funcTrace() {
		TraceMgr.AddName(cpfi.ID, cpfi.GlobalName(), "application")
	}
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
	// note binding of pattern to dictionary map to function map.  
	ptnMap := CmpPtnMapDict.Map[cpfi.PtnName] 
	hostPri := ptnMap.FuncMap[cpfi.Label]
	if strings.Contains(hostPri, ",") {
		pieces := strings.Split(hostPri, ",")
		return pieces[0]
	}
	return hostPri
}

func CmpPtnFuncPriority(cpfi *CmpPtnFuncInst) int {
	ptnMap := CmpPtnMapDict.Map[cpfi.PtnName] // note binding of pattern to dictionary map to function map.  Shared needs something different
	hostPri := ptnMap.FuncMap[cpfi.Label]
	if strings.Contains(hostPri, ",") {
		pieces := strings.Split(hostPri, ",")
		pri, _ := strconv.Atoi(pieces[1])
		return pri
	}
	return 1.0
}
