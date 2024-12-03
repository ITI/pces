package pces

// desc-cp.go holds data structures and methods used primarily
// to build pces models for files and so is primarily used
// by separate programs that build models, and by pces methods
// that read in those models from file and transform them into
// data structures used at run-time by the simulator

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type funcDesc struct {
	CP    string
	Label string
}

func createFuncDesc(cp, label string) funcDesc {
	fd := new(funcDesc)
	fd.CP = cp
	fd.Label = label
	return *fd
}

// CompPattern is a directed graph that describes the data flow among functions that implement an end-to-end computation
type CompPattern struct {
	// a model may use a number of instances of CompPatterns that have the same CPType
	CPType string `json:"cptype" yaml:"cptype"`

	// per-instance name of the pattern template
	Name string `json:"name" yaml:"name"`

	// instances of functions indexed by unique-to-pattern label
	Funcs []Func `json:"funcs" yaml:"funcs"`

	Services map[string]funcDesc `json:"services" yaml:"services"`

	// description of edges in Pattern graph
	Edges []CmpPtnGraphEdge `json:"edges" yaml:"edges"`

	// description of external edges in Pattern graph
	ExtEdges []XCPEdge `json:"extedges" yaml:"extedges"`
}

// CreateCompPattern is an initialization constructor.
// Its output struct has methods for integrating data.
func CreateCompPattern(cmptnType string) *CompPattern {

	cp := &CompPattern{CPType: cmptnType, Funcs: make([]Func, 0), Edges: make([]CmpPtnGraphEdge, 0)}
	cp.ExtEdges = make([]XCPEdge, 0)

	cp.Funcs = make([]Func, 0)
	cp.Services = make(map[string]funcDesc)

	// make type the default name, possible over-ride later
	cp.SetName(cmptnType)

	return cp
}

// DeepCopy creates a copy of CompPattern that explicitly copies various complex data structures
func (cpt *CompPattern) DeepCopy() *CompPattern {
	ncp := new(CompPattern)
	ncp.CPType = cpt.CPType
	ncp.Name = cpt.Name
	ncp.Funcs = make([]Func, len(cpt.Funcs))
	for idx, f := range cpt.Funcs {
		ncp.Funcs[idx] = Func{Class: f.Class, Label: f.Label}
	}

	ncp.Services = make(map[string]funcDesc)
	for key, value := range cpt.Services {
		ncp.Services[key] = value
	}

	ncp.Edges = make([]CmpPtnGraphEdge, len(cpt.Edges))
	for idx, e := range cpt.Edges {
		ncp.Edges[idx] = CmpPtnGraphEdge{SrcLabel: e.SrcLabel, MsgType: e.MsgType,
			DstLabel: e.DstLabel}
	}

	ncp.ExtEdges = make([]XCPEdge, len(cpt.ExtEdges))
	for idx, xe := range cpt.ExtEdges {
		ncp.ExtEdges[idx] =
			XCPEdge{SrcCP: xe.SrcCP,
				DstCP:    xe.DstCP,
				SrcLabel: xe.SrcLabel,
				DstLabel: xe.DstLabel,
				MsgType:  xe.MsgType}
	}
	return ncp
}

// local dictionary that gives access to a CompPattern give its name
var cmptnByName map[string]*CompPattern = make(map[string]*CompPattern)

// AddFunc includes a function specification to a CompPattern
func (cpt *CompPattern) AddFunc(fs *Func) {
	// make sure that the class declared for the function exists
	_, present := ClassMethods[fs.Class]
	if !present {
		panic(fmt.Errorf("function %s declares unrecognized class %s", fs.Label, fs.Class))
	}
	cpt.Funcs = append(cpt.Funcs, *fs)
}

// AddService includes a function specification to a CompPattern
func (cpt *CompPattern) AddService(srvName, srvFunc string) {
	// make sure that the class declared for the function exists
	for _, srvf := range cpt.Funcs {
		if srvf.Label == srvFunc {
			cpt.Services[srvName] = createFuncDesc(cpt.Name, srvFunc)
			return
		}
	}
	panic(fmt.Errorf("unrecognized func name %s", srvFunc))
}

// AddEdge creates an edge that describes message flow from one Func to another in the same comp pattern
// and adds it to the CompPattern's list of edges. Called from code that is building a model, applies some
// sanity checking
func (cpt *CompPattern) AddEdge(srcFuncLabel, dstFuncLabel string, msgType string,
	msgs *[]CompPatternMsg) {

	pe := CmpPtnGraphEdge{SrcLabel: srcFuncLabel, DstLabel: dstFuncLabel, MsgType: msgType}

	// look for duplicated edge
	for _, edge := range cpt.Edges {
		if pe == edge {
			return
		}
	}

	// look for duplicated message type for edges with the same destination
	for _, edge := range cpt.Edges {
		if edge.DstLabel == dstFuncLabel && msgType == edge.MsgType {
			panic(fmt.Errorf("%s declares identical message type %s directed to destination %s",
				cpt.Name, msgType, dstFuncLabel))
		}
	}

	// ensure that both function labels given have been created and
	// added to the CompPattern already
	srcFound := false
	dstFound := false

	for _, fnc := range cpt.Funcs {
		if fnc.Label == srcFuncLabel {
			srcFound = true
		}
		if fnc.Label == dstFuncLabel {
			dstFound = true
		}
	}

	// panic if either src or dst func is found
	if !srcFound || !dstFound {
		panic(fmt.Errorf("call to CmpPtn %s AddEdge specifies undeclared function", cpt.Name))
	}

	// check whether message type was added to the CompPatterns cpInit dictionary
	// whose Msgs list is an argument to this call
	msgTypeFound := false
	for _, msg := range *msgs {
		if msgType == msg.MsgType {
			msgTypeFound = true
			break
		}
	}

	// panic if the message type has not (yet) been declared
	if !msgTypeFound {
		panic(fmt.Errorf("message type %s in AddEdge for CmpPtn %s not yet declared", msgType, cpt.Name))
	}

	// passed all the checks above, so include the edge
	cpt.Edges = append(cpt.Edges, pe)
}

// XCPEdge describes an edge between different CmpPtns.
//
//	These are always rooted in a function of the chgCP class,
//
// where they are organized in a map whose index is the ultimate target CP for a message.
// The attribute is an XCPEdge, which specifies (a) the identity of the next CmpPtn,
// (b) the identity of the function to receive the message, (c) the type of the X-CP message, and
// (d) the methodCode for the function method to be executed.   Note that this structure
// limits one XCPEdge per chgCP instance per target CP.
type XCPEdge struct {
	SrcCP    string
	DstCP    string
	SrcLabel string
	DstLabel string
	MsgType  string
}

func extEdgeEq(xe1 *XCPEdge, xe2 *XCPEdge) bool {
	if (xe1.SrcCP != xe2.SrcCP) || (xe1.DstCP != xe2.DstCP) {
		return false
	}

	if (xe1.SrcLabel != xe2.SrcLabel) || (xe1.DstLabel != xe2.DstLabel) {
		return false
	}

	if xe1.MsgType != xe2.MsgType {
		return false
	}
	return true
}

// AddExtEdge creates an edge that describes message flow from one Func to another in
// a different computational pattern and adds it to the CompPattern's list of external edges.
// Perform some sanity checks before commiting the edge
func (cpt *CompPattern) AddExtEdge(srcCP, dstCP, srcLabel, dstLabel string, msgType string,
	srcMsgs *[]CompPatternMsg, dstMsgs *[]CompPatternMsg) {

	// create the edge we'll commit if it passes sanity checks
	pxe := XCPEdge{SrcCP: srcCP, DstCP: dstCP, SrcLabel: srcLabel, DstLabel: dstLabel, MsgType: msgType}

	// N.B. when AddExtEdge is called the model is being built and we do not yet have
	// instances of the CmpPtn, so index on the string name rather than the instance id
	// look for duplicated edge

	for _, xedge := range cpt.ExtEdges {
		if extEdgeEq(&pxe, &xedge) {
			return
		}
	}
	cpt.ExtEdges = append(cpt.ExtEdges, pxe)
}

// SetName copies the given name to be the CmpPtn's attribute and
// saves the name -> CmpPtn mapping in cmptnByName
func (cpt *CompPattern) SetName(name string) {
	cpt.Name = name
	cmptnByName[name] = cpt
}

// CompPatternDict holds pattern descriptions, is serializable
type CompPatternDict struct {
	DictName string                 `json:"dictname" yaml:"dictname"`
	Patterns map[string]CompPattern `json:"patterns" yaml:"patterns"`
}

// CreateCompPatternDict is an initialization constructor.
// Its output struct has methods for integrating data.
func CreateCompPatternDict(name string) *CompPatternDict {
	cpd := new(CompPatternDict)
	cpd.DictName = name
	cpd.Patterns = make(map[string]CompPattern)

	return cpd
}

// RecoverCompPattern returns a copy of a CompPattern from the dictionary,
// indexing by type, and applying a name
func (cpd *CompPatternDict) RecoverCompPattern(cptype string, cpname string) (*CompPattern, bool) {
	key := cpname

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
func (cpd *CompPatternDict) AddCompPattern(ptn *CompPattern) error {
	// warn if we're overwriting an existing entry?
	_, present := cpd.Patterns[ptn.Name]
	if present {
		return fmt.Errorf("overwrite pattern name %s in dictionary %s", ptn.Name, cpd.DictName)
	}

	// save a copy, not a pointer to a struct
	cpd.Patterns[ptn.Name] = *ptn
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

// GlobalFuncID is a global identifier for a function,
// naming the CmpPtn that holds it and its label within that CmpPtn
type GlobalFuncID struct {
	CmpPtnName string
	Label      string
}

// SharedCfgGroup gathers descriptions of functions that share
// the same cfg information, even across CmpPtn boundaries
type SharedCfgGroup struct {
	Name      string         // give a name to this shared cfg group
	Class     string         // all members have to be in the same class
	Instances []GlobalFuncID // slice identifying the representations that share cfg
	CfgStr    string         // the configuration they share, used at initialization
}

// CreateSharedCfgGroup is a constructor
func CreateSharedCfgGroup(name string, class string) *SharedCfgGroup {
	ssg := new(SharedCfgGroup)
	ssg.Name = name
	ssg.Class = class
	ssg.Instances = make([]GlobalFuncID, 0)
	return ssg
}

// AddInstance appends a global function description to
// a shared cfg group, but makes sure that it does not exist
// already in that group
func (ssg *SharedCfgGroup) AddInstance(cmpPtnName, label string) {

	for _, gfid := range ssg.Instances {
		if cmpPtnName == gfid.CmpPtnName && label == gfid.Label {
			fmt.Printf("Warning, attempt to add duplicated global function id to shared cfg group %s\n", ssg.Name)
			return
		}
	}
	gfid := GlobalFuncID{CmpPtnName: cmpPtnName, Label: label}
	ssg.Instances = append(ssg.Instances, gfid)
}

// AddCfg gives a shared cfg group a serialized common cfg
func (ssg *SharedCfgGroup) AddCfg(cfgStr string) {
	ssg.CfgStr = cfgStr
}

// SharedCfgGroupList holds all the shared cfg groups defined,
// for inclusion in a shared cfg description file
type SharedCfgGroupList struct {
	// UseYAML flags whether to interpret the seriaized cfg using json or yaml
	UseYAML bool             `json:"useyaml" yaml:"useyaml"`
	Groups  []SharedCfgGroup `json:"groups" yaml:"groups"`
}

// CreateSharedCfgGroupList is a constructor
func CreateSharedCfgGroupList(yaml bool) *SharedCfgGroupList {
	scgl := new(SharedCfgGroupList)
	scgl.UseYAML = yaml
	scgl.Groups = make([]SharedCfgGroup, 0)
	return scgl
}

// AddSharedCfgGroup includes an offered cfg group the the list,
// but checks that there is not already one there with the same name and class
func (scgl *SharedCfgGroupList) AddSharedCfgGroup(ssg *SharedCfgGroup) {
	for _, ssgrp := range scgl.Groups {
		if ssgrp.Name == ssg.Name && ssgrp.Class == ssg.Class {
			panic(fmt.Errorf("attempt to include shared cfg class with same name %s and class	%s as previously included",
				ssg.Name, ssg.Class))
		}
	}
	scgl.Groups = append(scgl.Groups, *ssg)
}

// ReadSharedCfgGroupList returns a deserialized slice of bytes into a SharedCfgGroupList.  Bytes are either provided, or are
// read from a file whose name is given.
func ReadSharedCfgGroupList(filename string, useYAML bool, dict []byte) (*SharedCfgGroupList, error) {
	var err error

	// empty slice of bytes means we get those bytes from the named file
	if len(dict) == 0 {
		// validate input file name
		fileInfo, err := os.Stat(filename)
		if os.IsNotExist(err) || fileInfo.IsDir() {
			msg := fmt.Sprintf("shared cfg group list file %s does not exist or cannot be read", filename)
			fmt.Println(msg)
			return nil, errors.New(msg)
		}
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	example := SharedCfgGroupList{}

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

// WriteToFile serializes the SharedCfgGroupList and writes it to a file.  Output file
// extension identifies whether serialization is to json or to yaml
func (scgl *SharedCfgGroupList) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*scgl)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*scgl, "", "\t")
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
	// name of the Computation Pattern whose Funcs are being initialized
	Name string `json:"name" yaml:"name"`

	// CPType of CP being initialized
	CPType string `json:"cptype" yaml:"cptype"`

	// UseYAML flags whether to interpret the seriaized initialization structure using json or yaml
	UseYAML bool `json:"useyaml" yaml:"useyaml"`

	// Cfg is indexed by Func label, mapping to a serialized representation of a struct
	Cfg map[string]string `json:"cfg" yaml:"cfg"`

	// Msgs holds a list of CompPatternMsgs used between Funcs in a CompPattern
	Msgs []CompPatternMsg `json:"msgs" yaml:"msgs"`
}

// CreateCPInitList constructs a CPInitList for an instance of a CompPattern
// and initializes it with a name and flag indicating whether YAML is used
func CreateCPInitList(name string, cptype string, useYAML bool) *CPInitList {
	cpil := new(CPInitList)
	cpil.Name = name
	cpil.CPType = cptype
	cpil.UseYAML = useYAML
	cpil.Cfg = make(map[string]string)

	cpil.Msgs = make([]CompPatternMsg, 0)
	return cpil
}

// DeepCopy creates a copy of CPInitList that explicitly copies various complex data structures
func (cpil *CPInitList) DeepCopy() *CPInitList {
	nl := new(CPInitList)
	nl.Name = cpil.Name
	nl.CPType = cpil.CPType
	nl.UseYAML = cpil.UseYAML
	nl.Cfg = make(map[string]string)
	for k, v := range cpil.Cfg {
		nl.Cfg[k] = v
	}
	nl.Msgs = make([]CompPatternMsg, len(cpil.Msgs))
	for idx, msg := range cpil.Msgs {
		nl.Msgs[idx] = CompPatternMsg{MsgType: msg.MsgType,
			IsPckt: msg.IsPckt}
	}
	return nl
}

// AddCfg puts a serialized initialization struct in the dictionary indexed by Func label
func (cpil *CPInitList) AddCfg(cpt *CompPattern, fnc *Func, cfg string) {
	// make sure that the function to which the cfg is attached has been defined for the given CmpPtn
	foundFunc := false
	for _, cpFunc := range cpt.Funcs {
		if cpFunc.Label == fnc.Label {
			foundFunc = true
			break
		}
	}
	if !foundFunc {
		panic(fmt.Errorf("attempt to add cfg to CmpPtn %s for a function %s not defined", cpt.Name, fnc.Label))
	}
	cpil.Cfg[fnc.Label] = cfg
}

// AddMsg appends description of a ComPatternMsg to the CPInitList's slice of messages used by the CompPattern.
// An error is returned if the msg's type already exists in the Msgs list
func (cpil *CPInitList) AddMsg(msg *CompPatternMsg) error {
	for _, xmsg := range cpil.Msgs {
		if xmsg.MsgType == msg.MsgType {
			return fmt.Errorf("message type %s for CP label %s has duplicate definition", msg.MsgType, cpil.Name)
		}
	}
	cpil.Msgs = append(cpil.Msgs, *msg)
	return nil
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
			return nil, errors.New(msg)
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
	DictName string `json:"dictname" yaml:"dictname"`

	// indexed by name of comp pattern
	InitList map[string]CPInitList `json:"initlist" yaml:"initlist"`
}

// CreateCPInitListDict is an initialization constructor.
// Its output struct has methods for integrating data.
func CreateCPInitListDict(name string) *CPInitListDict {
	cpild := new(CPInitListDict)
	cpild.DictName = name
	cpild.InitList = make(map[string]CPInitList)
	return cpild
}

// RecoverCPInitList returns a copy of a CPInitList from the dictionary
func (cpild *CPInitListDict) RecoverCPInitList(cptype string, cpname string) (*CPInitList, bool) {
	cpil, present := cpild.InitList[cpname]
	if !present {
		return nil, false
	}
	return &cpil, true
}

// AddCPInitList puts a CPInitList into the dictionary
func (cpild *CPInitListDict) AddCPInitList(cpil *CPInitList) error {
	_, present := cpild.InitList[cpil.Name]
	if present {
		return fmt.Errorf("attempt to overwrite comp pattern initialization list %s", cpild.DictName)
	}

	cpild.InitList[cpil.Name] = *cpil
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
			return nil, errors.New(msg)
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

// CompPatternMsg defines the structure of identification of messages that pass between Funcs in a CompPattern.
// Structures of this sort are transformed by a simulation run into a form that include experiment-defined payloads,
// and so representation of payload is absent here,
type CompPatternMsg struct {
	// edges in the CompPattern graph are labeled with MsgType, which means that a message across the edge must match in this attribute
	MsgType string `json:"msgtype" yaml:"msgtype"`

	// a message may be a packet or a flow
	IsPckt bool `json:"ispckt" yaml:"ispckt"`
}

// CreateCompPatternMsg is a constructer.
func CreateCompPatternMsg(msgType string, isPckt bool) *CompPatternMsg {
	cpm := new(CompPatternMsg)
	cpm.MsgType = msgType
	cpm.IsPckt = isPckt
	return cpm
}

// An InEdge describes the source Func of an incoming edge, the type of message it carries, and the method code
// flagging what code should execute as a result
type InEdge struct {
	SrcLabel string `json:"srclabel" yaml:"srclabel"`
	MsgType  string `json:"msgtype" yaml:"msgtype"`
}

// An OutEdge describes the destination Func of an outbound edge, and the type of message it carries.
type OutEdge struct {
	MsgType  string `json:"msgtype" yaml:"msgtype"`
	DstLabel string `json:"dstlabel" yaml:"dstlabel"`
}

// A Func represents a function used within a [CompPattern].
// Its 'Label' attribute is an identifier for an instance of the Func that is unique among all Funcs
// that make up a CompPattern which uses it, and the Class attribute is an identifier used when Func describes are
// stored in a dictionary before being copied and assembled as part of CompPattern construction. Class typically describes
// the computation the Func represents.
type Func struct {
	// identifies function, e.g., encryptRSA, used to look up execution time
	Class string `json:"class" yaml:"class"`

	// particular name given to function instance within a CompPattern
	Label string `json:"label" yaml:"label"`
}

// CreateFunc is a constructor for a [Func].  All parameters are given:
//   - Class, a string identifying what instances of this Func do.  Like a variable type.
//   - FuncLabel, a unique identifier (within an instance of a [CompPattern]) of an instance of this Func
func CreateFunc(class, funcLabel string) *Func {
	// see whether class is recognized
	_, present := ClassMethods[class]
	if !present {
		panic(fmt.Errorf("function class %s not recognized", class))
	}
	fd := &Func{Class: class, Label: funcLabel}

	return fd
}

// ReportErrs transforms a list of errors and transforms the non-nil ones into a single error
// with comma-separated report of all the constituent errors, and returns it.
func ReportErrs(errs []error) error {
	errMsg := make([]string, 0)
	for _, err := range errs {
		if err != nil {
			errMsg = append(errMsg, err.Error())
		}
	}
	if len(errMsg) == 0 {
		return nil
	}

	return errors.New(strings.Join(errMsg, ","))
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

// CheckReadableFiles probles the file system to ensure that every
// one of the argument filenames exists and is readable
func CheckReadableFiles(names []string) (bool, error) {
	return CheckFiles(names, true)
}

// CheckOutputFiles probles the file system to ensure that every
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
