package mrnesbits

// file cpg-static.go instantiates the cmpPtnFunc interface for the 'static' execution model

// The cmpPtnStaticFuncInst struct represents an instantiated instance of a function constrained to the 'static' execution model.
type cmpPtnStaticFuncInst struct {
	period  float64 // for a self-initiating func 'period' is non-zero, and gives the time between successive self-initiations
	label   string  // an identifier for this func, unique within the instance of CompPattern holding it.  May be reused in different CompPattern definitions.
	host    string  // identity of the host to which this func is mapped for execution
	fType   string  // 'function type', copied from the 'FuncType' attribute of the desc function upon which this instance is based
	ptnName string  // name of the instantiated CompPattern holding this function
	id      int     // integer identity which is unique among all objects in the MrNesbits model

	// at run-time start-up this function is initialized with all its responses.   For the static execution model,
	// the edge over which a response message is sent is statically determined by the edge over which the message that
	// triggers the response is received.  The 'resp' list holds all of this func's static assignments.
	resp map[InEdgeStruct]OutEdgeStruct
}

// createStaticFuncInst is a constructor that builds an instance of cmpPtnStaticFunctInst from a Func description and
// a serialized representation of a StaticParameters struct.
func createStaticFuncInst(cpInstName string, fnc *Func, paramStr string, useYAML bool) *cmpPtnStaticFuncInst {
	sfi := new(cmpPtnStaticFuncInst)
	sfi.id = nxtId()         // get an integer id that is unique across all objects in the simulation model
	sfi.fType = fnc.FuncType // remember the function type of the Func which is the base for this instance
	sfi.label = fnc.Label    // remember a label given to this function instance as part of building a CompPattern graph
	sfi.ptnName = cpInstName // remember the name of the instance of the comp pattern in which this func resides

	// deserialize the encoded StaticParameters struct
	params, _ := DecodeStaticParameters(paramStr, useYAML)

	// we'll reset period if the static parameters initialization struct tells us to
	sfi.period = 0.0

	// look up the func-to-host assignment, established earlier in the initialization sequence
	sfi.host = cmpPtnFuncHost(sfi)

	// look for a response that indicates self-initiation.  Detected when an edge's srcLabel is the same as this func's label (meaning self->self)
	for _, resp := range params.Response {
		if resp.InEdge.SrcLabel == sfi.label {
			// found one, save the period
			sfi.period = resp.Period

			// while a cmpPtnInst may have multiple funcs that self-initiate, a given func has at most one self initiation arc, so we're done
			break
		}
	}

	// initialize the slice of responses
	sfi.resp = make(map[InEdgeStruct]OutEdgeStruct)

	// edges are maps of message type to func node labels. Here we copy the input list, but
	// gather together edges that have a common InEdge  (N.B., InEdge is a structure more complex than an int or string, but can be used to index maps)
	for _, resp := range params.Response {
		sfi.resp[resp.InEdge] = resp.OutEdge
	}

	// we're done
	return sfi
}

// funcType gives the FuncType declares on the Fun upon which this instance is based
func (cpfsi *cmpPtnStaticFuncInst) funcType() string {
	return cpfsi.fType
}

// funcLabel gives the unique-within-cmpPtnInst label identifier for the function
func (cpfsi *cmpPtnStaticFuncInst) funcLabel() string {
	return cpfsi.label
}

// funcXtype returns the enumerated type 'static' to indicate that's this cmpPtnFunc's execution type
func (cpfsi *cmpPtnStaticFuncInst) funcXtype() funcExecType {
	return static
}

// funcId returns the unique-across-model integer identifier for this cmpPtnFunc
func (cpfsi *cmpPtnStaticFuncInst) funcId() int {
	return cpfsi.id
}

// funcHost returns the name of the topological Host upon which this cmpPtnFunc executes
func (cpfsi *cmpPtnStaticFuncInst) funcHost() string {
	return cpfsi.host
}

// funcCmpPtn gives the name of the cmpPtnInst the cmpPtnFunc is part of
func (cpfsi *cmpPtnStaticFuncInst) funcCmpPtn() string {
	return cpfsi.ptnName
}

// func funcPeriod gives the delay (in seconds) between self-initiations, for cmpPtnFuncs that self-initiate.  Otherwise returns the default value, 0.0
func (cpsfi *cmpPtnStaticFuncInst) funcPeriod() float64 {
	return cpsfi.period
}

// func funcDelay returns the delay (in seconds) associated with one execution of the cmpPtnFunc
func (cpfsi *cmpPtnStaticFuncInst) funcDelay(msg *cmpPtnMsg) float64 {
	// funcExecTime looks up the execution time from a table, returning a value that depends on the Host's CPU
	// and the packet length of the triggering message.  funcExecTime extracts these parameters from the inputs provided
	return funcExecTime(cpfsi, msg)
}

// funcExec determines the response of the static function, based (solely) on the
// the InEdge of the triggering message (which is passed as an input).  The response is described
// by a slice of cmpPtnMsgs.  For a static function this slice has exactly one element.
func (cpfsi *cmpPtnStaticFuncInst) funcExec(msg *cmpPtnMsg) []*cmpPtnMsg {
	// extract the source label and message type from the message edge
	srcLabel := msg.edge.srcLabel
	msgType := msg.edge.msgType

	// create an index from these, to access the response (outEdge) this particular instance of the cmpPtnFunc is initialized to provide.
	ies := InEdgeStruct{SrcLabel: srcLabel, MsgType: msgType}
	oes := cpfsi.resp[ies]

	// return the response by modifying the 'edge' attribute of the message to reflect the potentially different message type, and
	// (possibly empty) destination label
	msg.edge.srcLabel = cpfsi.funcLabel()
	msg.edge.msgType = oes.MsgType
	msg.edge.dstLabel = oes.DstLabel

	// return exactly one cmpPtnMsg
	return []*cmpPtnMsg{msg}
}
