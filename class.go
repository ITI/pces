package mrnesbits

import (
	"fmt"
	"github.com/iti/evt/evtm"
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
	End evtm.EventHandlerFunction
}

// Register is called to tell the system that a particular 
// function class exists, and gives a point to its description.
// The idea is to register only those function classes actually used, or at least,
// provide clear separation between classes.  Return of bool allows call to RegisterFuncClass
// as part of a variable assignment outside of a function body
func RegisterFuncClass(fc FuncClass) bool {
	className := fc.Class()
	_, present := FuncClasses[className]
	if present {
		panic(fmt.Errorf("attempt to register function class %s after that name already registered\n", fc.Class()))
	}
	FuncClasses[className] = fc
	return true
}
	
// map that takes us from the name of a FuncClass
var FuncClasses map[string]FuncClass = make(map[string]FuncClass)

func UpdateMsg(msg *CmpPtnMsg, srcLabel, msgType, dstLabel, edgeLabel string) {
	msg.MsgType = msgType
	msg.Edge = CreateCmpPtnGraphEdge(srcLabel, msgType, dstLabel, edgeLabel)
}

func validFuncClass(class string) bool {
	_, present := FuncClasses[class]
	return present
}

