package mrnesbits

import (
	"fmt"
	"github.com/iti/evt/evtm"
	"golang.org/x/exp/slices"
)

// file cpf.go holds structs and methods related to comp functions

// The funcOut struct describes a possible output edge for a response, along with a topology-free label
// 'choice' that can be used in the response selection process.
type funcOutEdge struct {
	DstLabel string
	MsgType  string
	Choice   string
}

// The CmpPtnFuncInst struct represents an instantiated instance of a function
//
//	constrained to the 'statefulerminisitc' execution model.
type CmpPtnFuncInst struct {
	InitFunc evtm.EventHandlerFunction  // if not 'emptyInitFunc' call this to initialize the function	
	InitMsg *CmpPtnMsg  // message that is copied when this instance is used to initiate a chain of func evaluations
	class  string	 // specifier leading to specific state, entrance, and exit functions
	Label  string    // an identifier for this func, unique within the instance of CompPattern holding it.
	host    string   // identity of the host to which this func is mapped for execution
	PtnName string   // name of the instantiated CompPattern holding this function
	id      int      // integer identity which is unique among all objects in the MrNesbits model
	active  bool     // flag whether function is actively processing inputs
	State   any      // holds string-coded state for string-code state variable names
	Reflect bool	 // whether the direction of the cmpHdr be return
	InterarrivalDist string  // for self-initiation
	InterarrivalMean float64 // for self-initiation

	// at run-time start-up this function is initialized with its responses
	// the set of potential output edges may be constrained, but not statefulermined solely by the input edge as with the static model.
	// Here, for a given input edge, 'resp' holds a slice of possible responses.
	Resp map[InEdge][]funcOutEdge

	// buffer the output messages created by executing the function body, indexed by the execId of the
	// initiating message
	msgResp map[int][]*CmpPtnMsg

	// respMethods indexed by message type
	respMethods map[string]*respMethod

	// when a message comes in with a given type, what's the method code we use to guide the executing response
	methodCode map[string]string
}


// createDestFuncInst== is a constructor that builds an instance of CmpPtnFunctInst from a Func description and
// a serialized representation of a StaticParameters struct.
func createFuncInst(cpInstName string, fnc *Func, paramStr, stateStr string, useYAML bool) *CmpPtnFuncInst {
	cpfi := new(CmpPtnFuncInst)
	cpfi.id = nxtId()                    // get an integer id that is unique across all objects in the simulation model
	cpfi.Label = fnc.Label               // remember a label given to this function instance as part of building a CompPattern graph
	cpfi.PtnName = cpInstName            // remember the name of the instance of the comp pattern in which this func resides
	cpfi.InitFunc = nil					 // will be over-ridden if there is an initialization event scheduled later
	cpfi.InitMsg = nil					 // will be over-ridden if the initialization block indicates initiation possible
	cpfi.active = true
	cpfi.class = fnc.Class
	cpfi.Resp = make(map[InEdge][]funcOutEdge)
	cpfi.msgResp = make(map[int][]*CmpPtnMsg)
	cpfi.methodCode = make(map[string]string)
	cpfi.respMethods = make(map[string]*respMethod)
	
	// look up the func-to-host assignment, established earlier in the initialization sequence
	cpfi.host = CmpPtnFuncHost(cpfi)

	// get a pointer to the function's class
	fc := FuncClasses[fnc.Class]

	params, err := DecodeFuncParameters(paramStr, useYAML)
	if err != nil {
		panic(err)
	}

	fc.InitState(cpfi, params.State, useYAML)

	// look for a response that flags a non-default (empty) self initiation
	for _, resp := range params.Response {
		// fnc.Class was already validated

		if !slices.Contains(FuncClassNames[fnc.Class], resp.MethodCode) {
			panic(fmt.Errorf("function method code %s not recognized for function class %s\n", resp.MethodCode, fnc.Class))
		}
	}

	// N.B. copy of state map used to be here.  Replace it

	// edges are maps of message type to func node labels. Here we copy the input list, but
	// gather together responses that have a common InEdge  (N.B., InEdge is a structure more complex 
	// than an int or string, but can be used to index maps)
	for _, resp := range params.Response {
		_, present := cpfi.Resp[resp.InEdge]

		// first time with this InEdge initialize the slice of responses
		if !present {
			cpfi.Resp[resp.InEdge] = make([]funcOutEdge, 0)
		}

		// create a response from the input response record
		df := funcOutEdge{DstLabel: resp.OutEdge.DstLabel, MsgType: resp.OutEdge.MsgType, Choice: resp.Choice}
		_, present = cpfi.methodCode[resp.InEdge.MsgType]
		if present {
			panic(fmt.Errorf("message type %s maps to multiple method codes\n", resp.InEdge.MsgType))
		}

		cpfi.methodCode[resp.InEdge.MsgType] = resp.MethodCode
		cpfi.Resp[resp.InEdge] = append(cpfi.Resp[resp.InEdge], df)
	}
	traceMgr.AddName(cpfi.id, cpfi.GlobalName(), "application")

	return cpfi
}

func (cpfsi *CmpPtnFuncInst) GlobalName() string {
	return cpfsi.PtnName+":"+cpfsi.Label
}

// AddResponse stores the selected out message response from executing the function,
// to be released later.  Saving through cpfsi.Resp[execId] to account for concurrent overlapping executions
func (cpfsi *CmpPtnFuncInst) AddResponse(execId int, resp []*CmpPtnMsg) {
	cpfsi.msgResp[execId] = resp
}

// funcResp returns the saved list of function response messages associated
// the the response to the input msg, and removes it from the msgResp map
func (cpfi *CmpPtnFuncInst) funcResp(execId int) []*CmpPtnMsg {

	rtn, present := cpfi.msgResp[execId]
	if !present {
		panic(fmt.Errorf("unsuccessful resp recovery\n"))
	}
	delete(cpfi.msgResp, execId)

	return rtn
}

func (cpfi *CmpPtnFuncInst) isInitiating() bool {
	return cpfi.InitFunc != nil
}

func (cpfi *CmpPtnFuncInst) InitMsgParams(msgType string, msgLen, pcktLen int, rate float64) {
    edge := CmpPtnGraphEdge{SrcLabel: cpfi.Label, DstLabel: cpfi.Label, MsgType: "initiate"}
    cpfi.InitMsg = CreateCmpPtnMsg(edge, cpfi.Label, cpfi.Label, msgType, msgLen, pcktLen, rate, cpfi.PtnName, cpfi.PtnName, nil, 0)
}

// funcActive indicates whether the function is processing messages
func (cpfi *CmpPtnFuncInst) funcActive() bool {
	return cpfi.active
}

// funcClass gives the FuncClass declares on the Func upon which this instance is based
func (cpfi *CmpPtnFuncInst) funcClass() string {
	return cpfi.class
}

// funcLabel gives the unique-within-CmpPtnInst label identifier for the function
func (cpfi *CmpPtnFuncInst) funcLabel() string {
	return cpfi.Label
}

// funcId returns the unique-across-model integer identifier for this CmpPtnFunc
func (cpfi *CmpPtnFuncInst) funcId() int {
	return cpfi.id
}

// funcCmpPtn gives the name of the CmpPtnInst the CmpPtnFunc is part of
func (cpfi *CmpPtnFuncInst) funcCmpPtn() string {
	return cpfi.PtnName
}

// funcDevice returns the name of the topological Host upon which this CmpPtnFunc executes
func (cpfi *CmpPtnFuncInst) funcDevice() string {
	return cpfi.host
}

// func funcInitEvtHdlr gives the function to schedule to initialize the function instance
func (cpfi *CmpPtnFuncInst) funcInitEvtHdlr() evtm.EventHandlerFunction {
	return cpfi.InitFunc
}
	
// respMethod associates two RespFunc that implement a function's response,
// one when it starts, the other when it ends
type respMethod struct {
	Start StartMethod
	End evtm.EventHandlerFunction
}


// AddStartMethod associates a string response 'name' with a pair of methods used to 
// represent the start and end of the function's execution.   The default method
// for the end method is ExitFunc
// Return of bool allows call to RegisterFuncClass
// as part of a variable assignment outside of a function body
func (cpfsi *CmpPtnFuncInst) AddStartMethod(methodCode string, start StartMethod) bool {
	// if the name for the methods is already in use, panic
	_, present := cpfsi.respMethods[methodCode]
	if present {
		panic(fmt.Errorf("function method name %s already in use\n", methodCode))
	}

	rp := respMethod{Start: start, End: ExitFunc}
	cpfsi.respMethods[methodCode] = &rp
	return true
}

// AddEndMethod includes a customized End method to a previously defined respMethod
func (cpfsi *CmpPtnFuncInst) AddEndMethod(methodCode string, end evtm.EventHandlerFunction) bool {
	_, present := cpfsi.respMethods[methodCode]
	if !present {
		panic(fmt.Errorf("AddEndMethod requires methodCode %s to have been declared already\n", methodCode))
	}
	cpfsi.respMethods[methodCode].End = end
	return true
}

func CmpPtnFuncHost(cpfi *CmpPtnFuncInst) string {
	ptnMap := CmpPtnMapDict.Map[cpfi.PtnName]
	return ptnMap.FuncMap[cpfi.Label]
}

