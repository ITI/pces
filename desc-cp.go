// functions here can also write created ones to file
package mrnesbits

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// CompPatternDesc is a directed graph that describes the data flow among functions that implement an end-to-end computation
type CompPattern struct {
	// a model may use a number of instances of CompPatterns that have the same CPType
	CPType string `json:"cptype" yaml:"cptype"`

	// per-instance name of the pattern template
	Name string `json:"name" yaml:"name"`

	// instances of functions indexed by unique-to-pattern label
	Funcs []Func `json:"funcs" yaml:"funcs"`

	// description of edges in Pattern graph
	Edges []PatternEdge `json:"edges" yaml:"edges"`
}

// CreateCompPattern is an initialization constructor.
// Its output struct has methods for integrating data.
func CreateCompPattern(cmptnType string) *CompPattern {
	var cp *CompPattern = &CompPattern{CPType: cmptnType, Funcs: make([]Func, 0), Edges: make([]PatternEdge, 0)}
	cmptnByName[cmptnType] = cp
	
	return cp
}

var cmptnByName map[string]*CompPattern = make(map[string]*CompPattern)

// AddFunc includes a function specification to a CompPattern
func (cpt *CompPattern) AddFunc(fs *Func) {
	cpt.Funcs = append(cpt.Funcs, *fs)
}

func (cpt *CompPattern) EdgeLabel(srcLabel, dstLabel, msgType string) string {
	for _, edge := range cpt.Edges {
		if edge.SrcLabel == srcLabel && edge.DstLabel == dstLabel && edge.MsgType == msgType {
			return edge.EdgeLabel
		}
	}

	return ""
}

// AddEdge creates an edge that describes message flow from one Func to another
// and adds it to the CompPattern's list of edges
func (cpt *CompPattern) AddEdge(srcFuncLabel, dstFuncLabel string, msgType string, edgeLabel string) {
	pe := PatternEdge{SrcLabel: srcFuncLabel, DstLabel: dstFuncLabel, MsgType: msgType, EdgeLabel: edgeLabel}

	// look for duplicated edge
	for _, edge := range cpt.Edges {
		if pe == edge {
			return
		}
	}
	// include the edge
	cpt.Edges = append(cpt.Edges, pe)
}

// CompPatterDict holds pattern descriptions, is serializable
type CompPatternDict struct {
	Prebuilt bool                   `json:"prebuilt" yaml:"prebuilt"`
	DictName string                 `json:"dictname" yaml:"dictname"`
	Patterns map[string]CompPattern `json:"patterns" yaml:"patterns"`
}

// CreateCompPatternDict is an initialization constructor.
// Its output struct has methods for integrating data.
func CreateCompPatternDict(name string, preblt bool) *CompPatternDict {
	cpd := new(CompPatternDict)
	cpd.Prebuilt = preblt
	cpd.DictName = name
	cpd.Patterns = make(map[string]CompPattern)

	return cpd
}

// RecoverCompPattern returns a copy of a CompPattern from the dictionary,
// indexing by type, and applying a name
func (cpd *CompPatternDict) RecoverCompPattern(cptype string, cpname string) (*CompPattern, bool) {
	var key string = cptype
	if !cpd.Prebuilt {
		key = cpname
	}

	// do we have any using the named type? Note that cp is a copy of that stored in the dictionary
	cp, present := cpd.Patterns[key]

	if present {
		cp.Name = cpname

		return &cp, true
	}

	return nil, false
}

// AddCompPattern amends a CompPattern dictionary with another CompPattern.  The prb flag indicates this
// is saved to a 'pre-built' dictionary from which selected CompPatterns are recovered when building a model.
// CP names are not created until build time, so the key used to read/write a CompPattern from the Patterns dictionary
// is the CompPattern cptype when accessing a prb dictionary, and the name otherwise
// so the CP type is used as a key when true, otherwise the CP name is known and used. If requested, and error is
// returned if the comp pattern being added is a duplicate.
func (cpd *CompPatternDict) AddCompPattern(ptn *CompPattern, prb bool, overwrite bool) error {
	// warn if we're overwriting an existing entry?
	if !overwrite {
		present := false
		if !prb {
			_, present = cpd.Patterns[ptn.Name]
		} else {
			_, present = cpd.Patterns[ptn.CPType]
		}

		if present {
			return fmt.Errorf("Overwrite pattern name %s in dictionary %s\n", ptn.Name, cpd.DictName)
		}
	}

	// save a copy, not a pointer to a struct
	if !prb {
		cpd.Patterns[ptn.Name] = *ptn
	} else {
		cpd.Patterns[ptn.CPType] = *ptn
	}
	return nil
}

// ReadCompPatternDict returns the transformation of a slice of bytes into a CompPatternDict, reading these from file if necessary.
func ReadCompPatternDict(filename string, useYAML bool, dict []byte) (*CompPatternDict, error) {
	var err error

	// empty dict slice means 'read from file'
	if len(dict) == 0 {
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	// dict should be non-empty now
	example := CompPatternDict{}

	// Select whether we read in json or yaml
	if useYAML {
		err = yaml.Unmarshal(dict, &example)
	} else {
		err = json.Unmarshal(dict, &example)
	}

	if err != nil {
		return nil, err
	}
	return &example, nil
}

// WriteToFile serializes the comp pattern, and saves to the named file. Output file extension
// determines whether serialization is to json or yaml
func (cpd *CompPatternDict) WriteToFile(filename string) error {
	// file extension determines whether we write json or yaml
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*cpd)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*cpd, "", "\t")
	}

	if merr != nil {
		panic(merr)
	}

	f, cerr := os.Create(filename)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(string(bytes[:]))
	if werr != nil {
		panic(werr)
	}
	f.Close()
	return nil
}

// CPInitList describes configuration parameters for the Funcs of a CompPattern
type CPInitList struct {
	// label of the Func being initialized
	Name string `json:"name" yaml:"name"`

	// CPType of CP being initialized
	CPType string `json:"cptype" yaml:"cptype"`

	// UseYAML flags whether to interpret the seriaized initialization structure using json or yaml
	UseYAML bool `json:"useyaml" yaml:"useyaml"`

	// Params is indexed by Func label, mapping to a serialized representation of a struct
	Params map[string]string `json:"params" yaml:"params"`

	// Msgs holds a list of CompPatternMsgs used between Funcs in a CompPattern
	Msgs []CompPatternMsg `json:"msgs" yaml:"msgs"`
}

// CreateCPInitList constructs a CPInitList and initializes it with a name and flag indicating whether YAML is used
func CreateCPInitList(name string, cptype string, useYAML bool) *CPInitList {
	cpil := new(CPInitList)
	cpil.Name = name
	cpil.CPType = cptype
	cpil.UseYAML = useYAML
	cpil.Params = make(map[string]string)
	cpil.Msgs = make([]CompPatternMsg, 0)
	return cpil
}

// AddParam puts a serialized initialization struct in the dictionary indexed by Func label
func (cpil *CPInitList) AddParam(label, param string) {
	cpil.Params[label] = param
}

// AddMsg appends description of a ComPatternMsg to the CPInitList's slice of messages used by the CompPattern
func (cpil *CPInitList) AddMsg(msg *CompPatternMsg) {
	cpil.Msgs = append(cpil.Msgs, *msg)
}

// ReadCPInitList returns a deserialized slice of bytes into a CPInitList.  Bytes are either provided, or are
// read from a file whose name is given.
func ReadCPInitList(filename string, useYAML bool, dict []byte) (*CPInitList, error) {
	var err error

	// empty slice of bytes means we get those bytes from the named file
	if len(dict) == 0 {
		// validate input file name
		fileInfo, err := os.Stat(filename)
		if os.IsNotExist(err) || fileInfo.IsDir() {
			msg := fmt.Sprintf("func parameter list %s does not exist or cannot be read", filename)
			fmt.Println(msg)
			return nil, fmt.Errorf(msg)
		}
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	example := CPInitList{}

	if useYAML {
		err = yaml.Unmarshal(dict, &example)
		example.UseYAML = true
	} else {
		err = json.Unmarshal(dict, &example)
		example.UseYAML = false
	}

	if err != nil {
		return nil, err
	}
	return &example, nil
}

// WriteToFile serializes the CPInitList and writes it to a file.  Output file
// extension identifies whether serialization is to json or to yaml
func (cpil *CPInitList) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*cpil)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*cpil, "", "\t")
	}

	if merr != nil {
		panic(merr)
	}

	f, cerr := os.Create(filename)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(string(bytes[:]))
	if werr != nil {
		panic(werr)
	}
	f.Close()
	return nil
}

// CPInitListDict holds CPInitList structures in a dictionary that may hold
// prebuilt versions (in which case InitList is indexed by CompPattern type) or
// is holding init lists for CPs to be part of an experiment, so the CP name is known is used as the key
type CPInitListDict struct {
	Prebuilt bool   `json:"prebuilt" yaml:"prebuilt"`
	DictName string `json:"dictname" yaml:"dictname"`

	// indexed by name of comp pattern
	InitList map[string]CPInitList `json:"initlist" yaml:"initlist"`
}

// CreateCPInitListDict is an initialization constructor.
// Its output struct has methods for integrating data.
func CreateCPInitListDict(name string, preblt bool) *CPInitListDict {
	cpild := new(CPInitListDict)
	cpild.Prebuilt = preblt
	cpild.DictName = name
	cpild.InitList = make(map[string]CPInitList)
	return cpild
}

// RecoverCPInitList returns a copy of a CompPattern from the dictionary, using the Prebuilt flag to
// choose index key, and applys a name
func (cpild *CPInitListDict) RecoverCPInitList(cptype string, cpname string) (*CPInitList, bool) {
	var key string = cptype

	if !cpild.Prebuilt {
		key = cpname
	}

	cpil, present := cpild.InitList[key]
	if !present {
		return nil, false
	}

	cpil.Name = cpname
	return &cpil, true
}

// AddCPInitList puts a CPInitList into the dictionary, selectively warning
// if this would overwrite
func (cpild *CPInitListDict) AddCPInitList(cpil *CPInitList, overwrite bool) error {
	var key string = cpil.CPType
	if !cpild.Prebuilt {
		key = cpil.Name
	}

	if !overwrite {
		_, present := cpild.InitList[key]
		if present {
			return fmt.Errorf("Attempt to overwrite comp pattern initialization list %s\n", cpild.DictName)
		}
	}
	cpild.InitList[key] = *cpil

	return nil
}

// WriteToFile serializes the CPInitListDict and writes it to a file.  Output file
// extension identifies whether serialization is to json or to yaml
func (cpild *CPInitListDict) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*cpild)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*cpild, "", "\t")
	}

	if merr != nil {
		panic(merr)
	}

	f, cerr := os.Create(filename)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(string(bytes[:]))
	if werr != nil {
		panic(werr)
	}

	f.Close()

	return nil
}

// ReadCPInitListDict deserializes a slice of bytes and returns a CPInitListDict.  Bytes are either provided, or are
// read from a file whose name is given.
func ReadCPInitListDict(filename string, useYAML bool, dict []byte) (*CPInitListDict, error) {
	var err error

	if len(dict) == 0 {
		// validate input file name
		fileInfo, err := os.Stat(filename)
		if os.IsNotExist(err) || fileInfo.IsDir() {
			msg := fmt.Sprintf("func parameter list dictionary %s does not exist or cannot be read", filename)
			fmt.Println(msg)
			return nil, fmt.Errorf(msg)
		}
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}
	example := CPInitListDict{}
	if useYAML {
		err = yaml.Unmarshal(dict, &example)
	} else {
		err = json.Unmarshal(dict, &example)
	}

	if err != nil {
		return nil, err
	}

	return &example, nil
}

// DecodeStaticParameters deserializes and returns a "StaticParameters" structure from
// a string that encodes its serialization
func DecodeStaticParameters(paramStr string, useYAML bool) (*StaticParameters, error) {
	staticResp := StaticParameters{}
	var err error
	if useYAML {
		err = yaml.Unmarshal([]byte(paramStr), &staticResp)
	} else {
		err = json.Unmarshal([]byte(paramStr), &staticResp)
	}
	return &staticResp, err
}

// DecodeStatefulParameters deserializes and returns a "StatefulParameters" structure from
// a string that encodes its serialization
func DecodeStatefulParameters(paramStr string, useYAML bool) (*StatefulParameters, error) {
	statefulResp := StatefulParameters{}
	var err error
	if useYAML {
		err = yaml.Unmarshal([]byte(paramStr), &statefulResp)
	} else {
		err = json.Unmarshal([]byte(paramStr), &statefulResp)
	}
	return &statefulResp, err
}

// DecodeRndParameters deserializes and returns a "RndParameters" structure from
// a string that encodes its serialization
func DecodeRndParameters(paramStr string) (*RndParameters, error) {
	rndResp := RndParameters{}
	err := yaml.Unmarshal([]byte(paramStr), &rndResp)
	return &rndResp, err
}

// CompPatternMsg defines the structure of identification of messages that pass between Funcs in a CompPattern.
// Structures of this sort are transformed by a simulation run into a form that include experiment-defined payloads,
// and so representation of payload is absent here,
type CompPatternMsg struct {
	// edges in the CompPattern graph are labeled with MsgType, which means that a message across the edge must match in this attribute
	MsgType string `json:"msgtype" yaml:"msgtype"`

	// PcktLen is a parameter used by some functions to select their execution time.  Not the same as the length of the message carrying the packet
	PcktLen int `json:"pcktlen" yaml:"pcktlen"`

	// MsgLen is the total length of the message c
	MsgLen int `json:"msglen" yaml:"msglen"`
}

// CreateCompPatternMsg is a constructer.
func CreateCompPatternMsg(msgType string, pcktLen int, msgLen int) *CompPatternMsg {
	cpm := new(CompPatternMsg)
	cpm.MsgType = msgType
	cpm.PcktLen = pcktLen
	cpm.MsgLen = msgLen
	return cpm
}

// A PatternEdge exists between the Funcs whose labels are given as SrcLabel and DstLabel. The edge is denoted to have a message type,
// and also a 'weight' whose use and interpretation depends on the Funcs whose labels are given
type PatternEdge struct {
	SrcLabel  string `json:"srclabel" yaml:"srclabel"`
	DstLabel  string `json:"dstlabel" yaml:"dstlabel"`
	MsgType   string `json:"msgtype" yaml:"msgtype"`
	EdgeLabel string `json:"edgelabel" yaml:"edgelabel"`
}

// CreatePatternEdge is a constructor.
func CreatePatternEdge(srcLabel, dstLabel, msgType, edgeLabel string) *PatternEdge {
	pe := new(PatternEdge)
	pe.SrcLabel = srcLabel
	pe.DstLabel = dstLabel
	pe.EdgeLabel = edgeLabel
	pe.MsgType = msgType
	return pe
}

// A FuncExecDesc struct holds a description of a function timing.
// ExecTime is the time (in seconds), attributes it depends on are
//
//		ProcessorType - the CPU,
//	 PcktLen - number of bytes in data packet being operated on
type FuncExecDesc struct {
	FuncOp        string  `json:"funcop" yaml:"funcop"`
	ProcessorType string  `json:"processortype" yaml:"processortype"`
	PcktLen       int     `json:"pcktlen" yaml:"pcktlen"`
	ExecTime      float64 `json:"exectime" yaml:"exectime"`
}

// A FuncExecList holds a map (Times) whose key is the FuncType
// of a Func, and whose value is a list of FuncExecDescs
// associated with all Funcs of that FuncType
type FuncExecList struct {
	// ListName is an identifier for this collection of timings
	ListName string `json:"listname" yaml:"listname"`

	// Times key is FuncType of CompPattern function.
	// Value is list of function times for that type of function
	Times map[string][]FuncExecDesc `json:"times" yaml:"times"`
}

// CreateFuncExecList is an initialization constructor.
// Its output struct has methods for integrating data.
func CreateFuncExecList(listname string) *FuncExecList {
	fel := new(FuncExecList)
	fel.ListName = listname
	fel.Times = make(map[string][]FuncExecDesc)
	return fel
}

// WriteToFile stores the FuncExecList struct to the file whose name is given.
// Serialization to json or to yaml is selected based on the extension of this name.
func (fel *FuncExecList) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*fel)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*fel, "", "\t")
	}

	if merr != nil {
		panic(merr)
	}

	f, cerr := os.Create(filename)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(string(bytes[:]))
	if werr != nil {
		panic(werr)
	}
	f.Close()

	return werr
}

// ReadFuncExecList deserializes a byte slice holding a representation of an FuncExecList struct.
// If the input argument of dict (those bytes) is empty, the file whose name is given is read
// to acquire them.  A deserialized representation is returned, or an error if one is generated
// from a file read or the deserialization.
func ReadFuncExecList(filename string, useYAML bool, dict []byte) (*FuncExecList, error) {
	var err error

	// if the dict slice of bytes is empty we get them from the file whose name is an argument
	if len(dict) == 0 {
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	example := FuncExecList{}

	if useYAML {
		err = yaml.Unmarshal(dict, &example)
	} else {
		err = json.Unmarshal(dict, &example)
	}

	if err != nil {
		return nil, err
	}

	return &example, nil
}

// AddTiming takes the parameters of a FuncExecDesc, creates one, and adds it to the FuncExecList
func (fel *FuncExecList) AddTiming(funcType, procType string, pcktLen int, execTime float64) {
	_, present := fel.Times[funcType]
	if !present {
		fel.Times[funcType] = make([]FuncExecDesc, 0)
	}
	fel.Times[funcType] = append(fel.Times[funcType], FuncExecDesc{ProcessorType: procType, PcktLen: pcktLen, ExecTime: execTime})
}

// A Func represents a function used within a [CompPattern].  Its 'ExecType' attribute identifies the semantics
// of its execution, its 'Label' attribute is an identifier for an instance of the Func that is unique among all Funcs
// that make up a CompPattern which uses it, and the FuncType attribute is an identifier used when Func describes are
// stored in a dictionary before being copied and assembled as part of CompPattern construction. FuncType typically describes
// the computation the Func represents.
type Func struct {
	// identifies function, e.g., encryptRSA, used to look up execution time
	FuncType string `json:"functype" yaml:"functype"`

	// particular name given to function instance within a CompPattern
	Label string `json:"label" yaml:"label"`

	// "static", "stateful", "random" e.g., guiding interpretation of Parameter string
	ExecType string `json:"exectype" yaml:"exectype"`
}

// FuncExcTypes enumerates classes of Funcs, each with a separate set of semantics
//
//		"static" implements static input-to-output mapping of message types, with fixed execution time.
//	 "stateful" allows the function to choose an output edge and message contents as a function
//	    of the input message type and contents, and state that is saved associated with the Func.
//		"random" implements a mapping of variable input message type to randomly selected output message
//			types and potentially random execution time, as a function of input message type
var FuncExcTypes []string = []string{"static", "stateful", "random"}

// An InEdgeStruct describes the source Func of an incoming edge, and the type of message it carries.
type InEdgeStruct struct {
	SrcLabel string `json:"srclabel" yaml:"srclabel"`
	MsgType  string `json:"msgtype" yaml:"msgtype"`
}

// An OutEdgeStruct describes the destination Func of an outbound edge, and the type of message it carries.
type OutEdgeStruct struct {
	MsgType  string `json:"msgtype" yaml:"msgtype"`
	DstLabel string `json:"dstlabel" yaml:"dstlabel"`
}

// A Static Response maps an InEdgeStruct to an outEdgeStruct, which for a Func with static execution type
// means an input received on the InEdge generates in response a message on the OutEdge.  If the SrcLabel of the InEdge
// is identical to the DstLabel of the OutEdge, the response is for self-initiation by the Func.  In this case the
// value of the 'Period' attribute gives the length of time (in seconds) between successive initiations.
type StaticResp struct {
	InEdge  InEdgeStruct  `json:"inedge" yaml:"inedge"`
	OutEdge OutEdgeStruct `json:"outedge" yaml:"outedge"`

	// if > 0, the time between successive self-initiations
	Period float64 `json:"period" yaml:"period"`
}

// A StaticParameters struct holds configuration information about the behaviour
// of a Func in an instantiated CompPattern.  The name of that CompPattern is
// saved in the struct, as is the Label for the Func to be initialized in the CompPattern.
// The 'Response' list gathers all of the input-output responses this instantiation
// executes.
type StaticParameters struct {
	// name of pattern holding labeled func
	PatternName string `json:"patternname" yaml:"patternname"`

	// label of function the parameters apply to
	Label string `json:"label" yaml:"label"`

	Response []StaticResp `json:"response" yaml:"response"`
}

// CreateStaticParameters is a constructor that saves the label of a Func, saves  the
// identity of the CompPattern which contains it, and initializes a slice of input-output response descriptions
func CreateStaticParameters(ptn, label string) *StaticParameters {
	sp := new(StaticParameters)
	sp.PatternName = ptn
	sp.Label = label
	sp.Response = make([]StaticResp, 0)

	return sp
}

// AddResponse takes the parameter of a StaticResponse, creates one, and adds it to the StaticParameter's list of responses
func (sp *StaticParameters) AddResponse(srcLabel, inMsgType, outMsgType, dstLabel string, period float64) {
	inEdge := InEdgeStruct{SrcLabel: srcLabel, MsgType: inMsgType}
	outEdge := OutEdgeStruct{MsgType: outMsgType, DstLabel: dstLabel}
	sr := StaticResp{InEdge: inEdge, OutEdge: outEdge, Period: period}
	sp.Response = append(sp.Response, sr)
}

// Serialize returns a serialized representation of the StaticParameters struct, in either json or yaml
// form, depending on the input parameter 'useYAML'.  If the serialization generates an error it is returned
//
//	with an empty string as the serialization result.
func (sp *StaticParameters) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*sp)
	} else {
		bytes, merr = json.Marshal(*sp)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// A Stateful Response maps an InEdgeStruct to an outEdgeStruct, which for a Func with static execution type
// means an input received on the InEdge _may_ generate in response a message on the OutEdge.  If the SrcLabel of the InEdge
// is identical to the DstLabel of the OutEdge, the response is for self-initiation by the Func.  In this case the
// value of the 'Period' attribute gives the length of time (in seconds) between successive initiations.
// 'Choice is an identifier used by the Func's logic for selecting an output edge.
type StatefulResp struct {
	// input edge
	InEdge InEdgeStruct `json:"inedge" yaml:"inedge"`

	// potential output edge
	OutEdge OutEdgeStruct `json:"outedge" yaml:"outedge"`

	// if Period > 0, the time between successive self-initiations, in seconds.
	Period float64 `json:"period" yaml:"period"`

	// an identifier used by the logic which chooses an output edge.
	Choice string `json:"choice" yaml:"choice"`
}

// A StatefulParameters struct holds information for initializing a Func with Stateful execution type.
// It names the label of the Func within an instantiated CompPattern, and the name of that CompPattern.
// It holds a string-to-string map which when filled out carries to the Func representation a general
// vehicle for defining state variables and initializing them.   The 'FuncSelect' attribute contains
// a code that the Func uses to select from among different (user pre-defined) behaviors in response to a message.
// 'Response' is a list of input-output edge responses associated with a Func. N.B. that a given input edge
// may be specified in multiple responses.
type StatefulParameters struct {
	// Pattern is name of the CompPattern holding labeled node these parameters modify
	PatternName string `json:"patternname" yaml:"patternname"`

	// Label of function being initialized
	Label string `json:"label" yaml:"label"`

	// State variables and their initial values (string-encoded)
	State map[string]string `json:"state" yaml:"state"`

	// FuncSelect value selects from among different blocks of program code that implement a response to input
	// Allows this struct definition to be used for wide variety of simulator-developed responses
	FuncSelect string `json:"funcselect" yaml:"funcselect"`

	// Response holds a list of all responses the Func may make
	Response []StatefulResp `json:"response" yaml:"response"`
}

// CreateStatefulParameters is a constructor. Its arguments initialize all the struct attributes except
// for the slice of responses, which it just initializes.
func CreateStatefulParameters(ptn, label, funcSelect string, state map[string]string) *StatefulParameters {
	sp := new(StatefulParameters)
	sp.PatternName = ptn
	sp.Label = label
	sp.FuncSelect = funcSelect
	sp.State = make(map[string]string)
	sp.State["FuncSelect"] = funcSelect

	if len(state) > 0 {
		for key, value := range state {
			sp.State[key] = value
		}
	}
	sp.Response = make([]StatefulResp, 0)

	return sp
}

// AddResponse takes the parameters of a Stateful response, creates a StatefulResp struct,
// and includes it in the struct's list of responses
func (sp *StatefulParameters) AddResponse(srcLabel, inMsgType, outMsgType, dstLabel string, period float64) {
	inEdge := InEdgeStruct{SrcLabel: srcLabel, MsgType: inMsgType}
	outEdge := OutEdgeStruct{MsgType: outMsgType, DstLabel: dstLabel}

	// find the label of the output edge named in this response and record it as the choice
	cmptn := cmptnByName[sp.PatternName]
	choice := cmptn.EdgeLabel(sp.Label, dstLabel, outMsgType)

	sr := StatefulResp{InEdge: inEdge, OutEdge: outEdge, Period: period, Choice: choice}
	sp.Response = append(sp.Response, sr)
}

// Serialize returns a serialized representation of the StatefulParameters struct, in either json or yaml
// form, depending on the input parameter 'useYAML'.  If the serialization generates an error it is returned
//
//	with an empty string as the serialization result.
func (sp *StatefulParameters) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	if useYAML {
		bytes, merr = yaml.Marshal(*sp)
	} else {
		bytes, merr = json.Marshal(*sp)
	}

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// A RndParameters struct is used to initialize a Func that has the "random" execution type.
type RndParameters struct {
	// Pattern is the name identifier of the [CompPattern] holding the Func being initialized
	PatternName string `json:"patternname" yaml:"patternname"`

	// Label is the label identifier of the [Func] being initialized
	Label string `json:"label" yaml:"label"`

	// Response holds all the possible responses the Func may make
	Response []RndResp `json:"response" yaml:"response"`
}

// CreateRndParameters is an initialization constructor.
// Its output struct has methods for integrating data.
func CreateRndParameters(ptn, label string) *RndParameters {
	rp := new(RndParameters)
	rp.PatternName = ptn
	rp.Label = label
	rp.Response = make([]RndResp, 0)

	return rp
}

// A RndResp struct describes a possible response to an incoming message.
// It identifies the input edge, and associates with that a map which
// represents a probability distribution to use when selecting an edge to push a response message through.
type RndResp struct {
	InEdge    InEdgeStruct              `json:"inedge" yaml:"inedge"`
	MsgSelect map[OutEdgeStruct]float64 `json:"msgselect" yaml:"msgselect"`
}

// AddResponse takes the parameters of a [RndResp], creates one, and includes it into the RndParameter's list
func (rp *RndParameters) AddResponse(srcLabel, inMsgType string, msgSelect map[OutEdgeStruct]float64) {
	inEdge := InEdgeStruct{SrcLabel: srcLabel, MsgType: inMsgType}
	rr := RndResp{InEdge: inEdge, MsgSelect: msgSelect}
	rp.Response = append(rp.Response, rr)
}

// Serialize creates a serialized form of the RndParameter. Always yaml
func (rp *RndParameters) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error

	// turns out that the structure of RndParameters limits its serialization to yaml
	/*
			if useYAML {
				bytes, merr = yaml.Marshal(*rp)
			} else {
		 		bytes, merr = json.MarshalIndex(*rp)
			}
	*/
	bytes, merr = yaml.Marshal(*rp)

	if merr != nil {
		return "", merr
	}

	return string(bytes[:]), nil
}

// CreateFunc is a constructor for a [Func].  All parameters are given:
//   - FuncType, a string identifying what instances of this Func do.  Like a variable type.
//   - FuncLabel, a unique identifier (within an instance of a [CompPattern]) of an instance of this Func
//   - ExecType, identifies semantics of responding to an input message
func CreateFunc(funcType, funcLabel, execType string) *Func {
	fd := &Func{FuncType: funcType, ExecType: execType, Label: funcLabel}

	return fd
}

// SerialParameters returns a serialization of the input parameter struct given as an argument.
// It choose serialization function depending on the Func's ExecType, and creates json or yaml
// representation, depending on the 'useYAML ' input argument.
func (fd *Func) SerializeParameters(params any, useYAML bool) (string, error) {
	var err error = nil

	switch fd.ExecType {
	case "static":
		sp := params.(*StaticParameters)
		return sp.Serialize(useYAML)

	case "stateful":
		dp := params.(*StatefulParameters)
		return dp.Serialize(useYAML)

	case "random":
		rp := params.(*RndParameters)
		return rp.Serialize(useYAML)
	}

	return "", err
}

// A CompPatternMap describes how funcs in an instantiated [CompPattern]
// are mapped to hosts
type CompPatternMap struct {
	// PatternName identifies the name of the pattern instantiation being mapped
	PatternName string `json:"patternname" yaml:"patternname"`

	// mapping of func labels to hosts. Key is Label attribute of Func
	FuncMap map[string]string `json:"funcmap" yaml:"funcmap"`
}

// CreateCompPatternMap is a constructor.
func CreateCompPatternMap(ptnName string) *CompPatternMap {
	cpm := new(CompPatternMap)
	cpm.PatternName = ptnName
	cpm.FuncMap = make(map[string]string)

	return cpm
}

// AddMapping inserts into the CompPatternMap a binding of Func label to a host.  Optionally,
// an error might be returned if a binding of that Func has already been made, and is different.
func (cpm *CompPatternMap) AddMapping(funcLabel string, hostname string, overwrite bool) error {
	// only check for overwrite if asked to
	if !overwrite {
		host, present := cpm.FuncMap[funcLabel]
		if present && host != hostname {
			return fmt.Errorf("attempt to overwrite mapping of func label %s in %s\n",
				funcLabel, cpm.PatternName)
		}
	}

	// store the mapping
	cpm.FuncMap[funcLabel] = hostname

	return nil
}

// WriteToFile stores the CompPatternMap struct to the file whose name is given.
// Serialization to json or to yaml is selected based on the extension of this name.
func (cpm *CompPatternMap) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*cpm)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*cpm, "", "\t")
	}

	if merr != nil {
		panic(merr)
	}

	f, cerr := os.Create(filename)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(string(bytes[:]))
	if werr != nil {
		panic(werr)
	}
	f.Close()

	return werr
}

// ReadCompPatternMap deserializes a byte slice holding a representation of an CompPatternMap struct.
// If the input argument of dict (those bytes) is empty, the file whose name is given is read
// to acquire them.  A deserialized representation is returned, or an error if one is generated
// from a file read or the deserialization.
func ReadCompPatternMap(filename string, useYAML bool, dict []byte) (*CompPatternMap, error) {
	var err error

	// if the dict slice of bytes is empty we get them from the file whose name is an argument
	if len(dict) == 0 {
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	example := CompPatternMap{}

	if useYAML {
		err = yaml.Unmarshal(dict, &example)
	} else {
		err = json.Unmarshal(dict, &example)
	}

	if err != nil {
		return nil, err
	}

	return &example, nil
}

// A CompPatternMapDict holds copies of CompPatternMap structs in a map that is
// indexed by the PatternName of resident CompPatternMaps
type CompPatternMapDict struct {
	DictName string                    `json:"dictname" yaml:"dictname"`
	Map      map[string]CompPatternMap `json:"map" yaml:"map"`
}

// CreateCompPatternMapDict is a constructor.
// Saves the dictionary name and initializes the map of CompPatternMaps to-be-stored.
func CreateCompPatternMapDict(name string) *CompPatternMapDict {
	cpmd := new(CompPatternMapDict)
	cpmd.DictName = name
	cpmd.Map = make(map[string]CompPatternMap)

	return cpmd
}

// AddCompPatternMap includes in the dictionary a CompPatternMap that is provided as input.
// Optionally an error may be returned if an entry for the associated CompPattern exists already.
func (cpmd *CompPatternMapDict) AddCompPatternMap(cpm *CompPatternMap, overwrite bool) error {
	if !overwrite {
		_, present := cpmd.Map[cpm.PatternName]
		if present {
			return fmt.Errorf("attempt to overwrite mapping of %s to comp pattern map dictionary\n", cpm.PatternName)
		}
	}
	cpmd.Map[cpm.PatternName] = *cpm

	return nil
}

// RecoverCompPatternMap returns a CompPatternMap associated with the CompPattern named in the input parameters.
// It returns also a flag denoting whether the identified CompPattern has an entry in the dictionary.
func (cpmd *CompPatternMapDict) RecoverCompPatternMap(pattern string) (*CompPatternMap, bool) {
	cpm, present := cpmd.Map[pattern]
	if !present {
		return nil, false
	}
	cpm.PatternName = pattern

	return &cpm, true
}

// ReadCompPatternMapDict deserializes a byte slice holding a representation of an CompPatternMapDict struct.
// If the input argument of dict (those bytes) is empty, the file whose name is given is read
// to acquire them.  A deserialized representation is returned, or an error if one is generated
// from a file read or the deserialization.
func ReadCompPatternMapDict(filename string, useYAML bool, dict []byte) (*CompPatternMapDict, error) {
	var err error
	if len(dict) == 0 {
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	example := CompPatternMapDict{}
	if useYAML {
		err = yaml.Unmarshal(dict, &example)
	} else {
		err = json.Unmarshal(dict, &example)
	}

	if err != nil {
		return nil, err
	}

	return &example, nil
}

// WriteToFile stores the CompPatternMapDict struct to the file whose name is given.
// Serialization to json or to yaml is selected based on the extension of this name.
func (cpd *CompPatternMapDict) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*cpd)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*cpd, "", "\t")
	}

	if merr != nil {
		panic(merr)
	}

	f, cerr := os.Create(filename)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(string(bytes[:]))
	if werr != nil {
		panic(werr)
	}
	f.Close()

	return werr
}

// An ExpParameter struct describes an input to experiment configuration at run-time. It specified
//   - ParamObj identifies the kind of thing being configured : Switch, Router, Host, Interface, or Network
//   - Attribute identifies a class of objects of that type to which the configuration parameter should apply.
//     May be "*" for a wild-card, may be "name%%xxyy" where "xxyy" is the object's identifier, may be
//     a comma-separated list of other attributes, documented [here]
type ExpParameter struct {
	// Type of thing being configured
	ParamObj string `json:"paramObj" yaml:"paramObj"`

	// attribute identifier for this parameter
	Attribute string `json:"attribute" yaml:"attribute"`

	// ParameterType, e.g., "Bandwidth", "WiredLatency", "CPU"
	Param string `json:"param" yaml:"param"`

	// string-encoded value associated with type
	Value string `json:"value" yaml:"value"`
}

// CreateExpParameter is a constructor.  Completely fills in the struct with the [ExpParameter] attributes.
func CreateExpParameter(paramObj, attribute, param, value string) *ExpParameter {
	exptr := &ExpParameter{ParamObj: paramObj, Attribute: attribute, Param: param, Value: value}

	return exptr
}

// An ExpCfg structure holds all of the ExpParameters for a named experiment
type ExpCfg struct {
	// Name is an identifier for a group of [ExpParameters].  No particular interpretation of this string is
	// used, except as a referencing label when moving an ExpCfg into or out of a dictionary
	Name string `json:"expname" yaml:"expname"`

	// Parameters is a list of all the [ExpParameter] objects presented to the simulator for an experiment.
	Parameters []ExpParameter `json:"parameters" yaml:"parameters"`
}

// An ExpCfgDict is a dictionary that holds [ExpCfg] objects in a map indexed by their Name.
type ExpCfgDict struct {
	DictName string            `json:"dictname" yaml:"dictname"`
	Cfgs     map[string]ExpCfg `json:"cfgs" yaml:"cfgs"`
}

// CreateExpCfgDict is a constructor.  Saves a name for the dictionary, and initializes the slice of ExpCfg objects
func CreateExpCfgDict(name string) *ExpCfgDict {
	ecd := new(ExpCfgDict)
	ecd.DictName = name
	ecd.Cfgs = make(map[string]ExpCfg)

	return ecd
}

// AddExpCfg adds the offered ExpCfg to the dictionary, optionally returning
// an error if an ExpCfg with the same Name is already saved.
func (ecd *ExpCfgDict) AddExpCfg(ec *ExpCfg, overwrite bool) error {

	// allow for overwriting duplication?
	if !overwrite {
		_, present := ecd.Cfgs[ec.Name]
		if present {
			return fmt.Errorf("Attempt to overwrite template ExpCfg %s\n", ec.Name)
		}
	}
	// save it
	ecd.Cfgs[ec.Name] = *ec

	return nil
}

// RecoverExpCfg returns an ExpCfg from the dictionary, with name equal to the input parameter.
// It returns also a flag denoting whether the identified ExpCfg has an entry in the dictionary.
func (ecd *ExpCfgDict) RecoverExpCfg(name string) (*ExpCfg, bool) {
	ec, present := ecd.Cfgs[name]
	if present {
		return &ec, true
	}

	return nil, false
}

// WriteToFile stores the ExpCfgDict struct to the file whose name is given.
// Serialization to json or to yaml is selected based on the extension of this name.
func (ecd *ExpCfgDict) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*ecd)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*ecd, "", "\t")
	}

	if merr != nil {
		panic(merr)
	}

	f, cerr := os.Create(filename)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(string(bytes[:]))
	if werr != nil {
		panic(werr)
	}
	f.Close()

	return werr
}

// ReadExpCfgDict deserializes a byte slice holding a representation of an ExpCfgDict struct.
// If the input argument of dict (those bytes) is empty, the file whose name is given is read
// to acquire them.  A deserialized representation is returned, or an error if one is generated
// from a file read or the deserialization.
func ReadExpCfgDict(filename string, useYAML bool, dict []byte) (*ExpCfgDict, error) {
	var err error
	if len(dict) == 0 {
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	example := ExpCfgDict{}
	if useYAML {
		err = yaml.Unmarshal(dict, &example)
	} else {
		err = json.Unmarshal(dict, &example)
	}

	if err != nil {
		return nil, err
	}

	return &example, nil
}

// CreateExpCfg is a constructor. Saves the offered Name and initializes the slice of ExpParameters.
func CreateExpCfg(name string) *ExpCfg {
	expcfg := &ExpCfg{Name: name, Parameters: make([]ExpParameter, 0)}
	return expcfg
}

// ValidateParameter returns an error if the paramObj, attribute, and param values don't
// make sense taken together within an ExpParameter.
func ValidateParameter(paramObj, attribute, param string) error {
	// the paramObj string has to be recognized as one of the permitted ones (stored in list ExpParamObjs)
	if !slices.Contains(ExpParamObjs, paramObj) {
		return fmt.Errorf("paramater paramObj %s is not recognized\n", paramObj)
	}

	// Start the analysis of the attribute by splitting it by comma
	attrbList := strings.Split(attribute, ",")

	// every elemental attribute needs to be a name or "*", or recognized as a legitimate attribute
	// for the associated paramObj
	for _, attrb := range attrbList {
		// if name is present it is the only acceptable attribute in the comma-separated list
		if strings.Contains(attrb, "name%%") {
			if len(attrbList) != 1 {
				return fmt.Errorf("name paramater attribute %s paramObj %s is included with more attributes\n", attrb, paramObj)
			}
			// otherwise OK
			return nil
		}

		// if "*" is present it is the only acceptable attribute in the comma-separated list
		if strings.Contains(attrb, "*") {
			if len(attrbList) != 1 {
				return fmt.Errorf("name paramater attribute * paramObj %s is included with more attributes\n", paramObj)
			}

			// otherwise OK
			return nil
		}

		// otherwise check the legitmacy of the individual attribute.  Whole string is invalidate if one component is invalid.
		if !slices.Contains(ExpAttributes[paramObj], attrb) {
			return fmt.Errorf("paramater attribute %s is not recognized for paramObj %s\n", attrb, paramObj)
		}
	}

	// comma-separated attribute is OK, make sure the type of param is consistent with the paramObj
	if !slices.Contains(ExpParams[paramObj], param) {
		return fmt.Errorf("paramater %s is not recognized for paramObj %s\n", param, paramObj)
	}

	// it's all good
	return nil
}

// AddParameter accepts the four values in an ExpParameter, creates one, and adds to the ExpCfg's list.
// Returns an error if the parameters are not validated.
func (expcfg *ExpCfg) AddParameter(paramObj, attribute, param, value string) error {
	// validate the offered parameter values
	err := ValidateParameter(paramObj, attribute, param)
	if err != nil {
		return err
	}

	// create an ExpParameter with these values
	excp := CreateExpParameter(paramObj, attribute, param, value)

	// save it
	expcfg.Parameters = append(expcfg.Parameters, *excp)

	return nil
}

// WriteToFile stores the ExpCfg struct to the file whose name is given.
// Serialization to json or to yaml is selected based on the extension of this name.
func (dict *ExpCfg) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*dict)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*dict, "", "\t")
	}

	if merr != nil {
		panic(merr)
	}

	f, cerr := os.Create(filename)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(string(bytes[:]))
	if werr != nil {
		panic(werr)
	}
	f.Close()

	return werr
}

// ReadExpCfg deserializes a byte slice holding a representation of an ExpCfg struct.
// If the input argument of dict (those bytes) is empty, the file whose name is given is read
// to acquire them.  A deserialized representation is returned, or an error if one is generated
// from a file read or the deserialization.
func ReadExpCfg(filename string, useYAML bool, dict []byte) (*ExpCfg, error) {
	var err error

	if len(dict) == 0 {
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	example := ExpCfg{}
	if useYAML {
		err = yaml.Unmarshal(dict, &example)
	} else {
		err = json.Unmarshal(dict, &example)
	}

	if err != nil {
		return nil, err
	}

	return &example, nil
}

// ReportErrs transforms a list of errors and transforms the non-nil ones into a single error
// with comma-separated report of all the constituent errors, and returns it.
func ReportErrs(errs []error) error {
	err_msg := make([]string, 0)
	for _, err := range errs {
		if err != nil {
			err_msg = append(err_msg, err.Error())
		}
	}
	if len(err_msg) == 0 {
		return nil
	}

	return errors.New(strings.Join(err_msg, ","))
}

// CheckDirectories probes the file system for the existence
// of every directory listed in the list of files.  Returns a boolean
// indicating whether all dirs are valid, and returns an aggregated error
// if any checks failed.
func CheckDirectories(dirs []string) (bool, error) {
	// make sure that every directory name included exists
	failures := []string{}

	// for every offered (non-empty) directory
	for _, dir := range dirs {
		if len(dir) == 0 {
			continue
		}

		// look for a extension, if any.   Having one means not a directory
		ext := filepath.Ext(dir)

		// ext being empty means this is a directory, otherwise a path
		if ext != "" {
			failures = append(failures, fmt.Sprintf("%s not a directory", dir))

			continue
		}

		if _, err := os.Stat(dir); err != nil {
			failures = append(failures, fmt.Sprintf("%s not reachable", dir))

			continue
		}
	}
	if len(failures) == 0 {
		return true, nil
	}

	err := errors.New(strings.Join(failures, ","))

	return false, err
}

// CheckReadableFileNames probles the file system to ensure that every
// one of the argument filenames exists and is readable
func CheckReadableFiles(names []string) (bool, error) {
	return CheckFiles(names, true)
}

// CheckOpututFileNames probles the file system to ensure that every
// argument filename can be written.
func CheckOutputFiles(names []string) (bool, error) {
	return CheckFiles(names, false)
}

// CheckFiles probes the file system for permitted access to all the
// argument filenames, optionally checking also for the existence
// of those files for the purposes of reading them.
func CheckFiles(names []string, checkExistence bool) (bool, error) {
	// make sure that the directory of each named file exists
	errs := make([]error, 0)

	for _, name := range names {
		// skip non-existent files
		if len(name) == 0 || name == "/tmp" {
			continue
		}

		// split off the directory portion of the path
		directory, _ := filepath.Split(name)
		if _, err := os.Stat(directory); err != nil {
			errs = append(errs, err)
		}
	}

	// if required, check for the reachability and existence of each file
	if checkExistence {
		for _, name := range names {
			if _, err := os.Stat(name); err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) == 0 {
			return true, nil
		}

		rtnerr := ReportErrs(errs)
		return false, rtnerr
	}

	return true, nil
}

// FuncExecTypes is a list of all the Func execution types
var FuncExecTypes = []string{"static", "stateful", "random"}

// ExpParamObjs, ExpAttributes, and ExpParams hold descriptions of the types of objects
// that are initialized by an exp file, for each the attributes of the object that can be tested for to determine
// whether the object is to receive the configuration parameter, and the parameter types defined for each object type
var ExpParamObjs []string
var ExpAttributes map[string][]string
var ExpParams map[string][]string

// GetExpParamDesc returns ExpParamObjs, ExpAttributes, and ExpParams after ensuring that they have been build
func GetExpParamDesc() ([]string, map[string][]string, map[string][]string) {
	if ExpParamObjs == nil {
		ExpParamObjs = []string{"Switch", "Router", "Host", "Interface", "Network"}
		ExpAttributes = make(map[string][]string)
		ExpAttributes["Switch"] = []string{"model", "*"}
		ExpAttributes["Router"] = []string{"model", "wired", "wireless", "*"}
		ExpAttributes["Host"] = []string{"*"}
		ExpAttributes["Interface"] = []string{"Switch", "Host", "Router", "wired", "wireless", "*"}
		ExpAttributes["Network"] = []string{"wired", "wireless", "LAN", "WAN", "T3", "T2", "T1", "*"}
		ExpParams = make(map[string][]string)
		ExpParams["Switch"] = []string{"execTime", "buffer"}
		ExpParams["Route"] = []string{"execTime", "buffer"}
		ExpParams["Host"] = []string{"CPU"}
		ExpParams["Network"] = []string{"media", "latency", "bandwidth", "capacity"}
		ExpParams["Interface"] = []string{"media", "latency", "bandwidth", "packetSize"}
	}

	return ExpParamObjs, ExpAttributes, ExpParams
}
