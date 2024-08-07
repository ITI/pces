package pces

// file desc-timing.go holds structs, methods, and data structures related
// to expression and recovery of timings of computational functions and device
// operations

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
	"os"
	"path"
)

// A FuncExecDesc struct holds a description of a function timing.
// ExecTime is the time (in seconds), attributes it depends on are
//
//	 Identifier - a unique name for this func call
//	 Param - additional information, e.g., key length for crypto
//		CPUModel - the CPU,
//		PcktLen  - number of bytes in data packet being operated on
type FuncExecDesc struct {
	Identifier string  `json:"identifier" yaml:"identifier"`
	Param      string  `json:"param" yaml:"param"`
	CPUModel   string  `json:"CPUModel" yaml:"CPUModel"`
	PcktLen    int     `json:"pcktlen" yaml:"pcktlen"`
	ExecTime   float64 `json:"exectime" yaml:"exectime"`
}

// A FuncExecList holds a map (Times) whose key is the class
// of a Func, and whose value is a list of FuncExecDescs
// associated with all Funcs of that class
type FuncExecList struct {
	// ListName is an identifier for this collection of timings
	ListName string `json:"listname" yaml:"listname"`

	// Times key is an identifier for the function.
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
func (fel *FuncExecList) AddTiming(identifier, param, cpumodel string,
	pcktLen int, execTime float64) {
	_, present := fel.Times[identifier]
	if !present {
		fel.Times[identifier] = make([]FuncExecDesc, 0)
	}
	fel.Times[identifier] = append(fel.Times[identifier],
		FuncExecDesc{Param: param, CPUModel: cpumodel,
			PcktLen: pcktLen, ExecTime: execTime, Identifier: identifier})
}
