package pces

// file desc-params.go holds structs, methods, and data structures used in specifying
// assignment of performance parameters to pces/mrnes models

import (
	"encoding/json"
	"fmt"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"strings"
)

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
			return fmt.Errorf("attempt to overwrite template ExpCfg %s", ec.Name)
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
		return fmt.Errorf("paramater paramObj %s is not recognized", paramObj)
	}

	// Start the analysis of the attribute by splitting it by comma
	attrbList := strings.Split(attribute, ",")

	// every elemental attribute needs to be a name or "*", or recognized as a legitimate attribute
	// for the associated paramObj
	for _, attrb := range attrbList {
		// if name is present it is the only acceptable attribute in the comma-separated list
		if strings.Contains(attrb, "name%%") {
			if len(attrbList) != 1 {
				return fmt.Errorf("name paramater attribute %s paramObj %s is included with more attributes", attrb, paramObj)
			}
			// otherwise OK
			return nil
		}

		// if "*" is present it is the only acceptable attribute in the comma-separated list
		if strings.Contains(attrb, "*") {
			if len(attrbList) != 1 {
				return fmt.Errorf("name paramater attribute * paramObj %s is included with more attributes", paramObj)
			}

			// otherwise OK
			return nil
		}

		// otherwise check the legitmacy of the individual attribute.  Whole string is invalidate if one component is invalid.
		if !slices.Contains(ExpAttributes[paramObj], attrb) {
			return fmt.Errorf("paramater attribute %s is not recognized for paramObj %s", attrb, paramObj)
		}
	}

	// comma-separated attribute is OK, make sure the type of param is consistent with the paramObj
	if !slices.Contains(ExpParams[paramObj], param) {
		return fmt.Errorf("paramater %s is not recognized for paramObj %s", param, paramObj)
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
func (expcfg *ExpCfg) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*expcfg)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*expcfg, "", "\t")
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

// ExpParamObjs holds descriptions of the types of objects
// that are initialized by an exp file for each the attributes of the object that can be tested for to determine
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
