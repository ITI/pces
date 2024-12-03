package pces

// desc-map.go holds structs, methods, and data structures used to define and then
// access mapping of a pces model's computational pattern function instances to
// CPUs in the underlaying mrnes computer and network model

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"strconv"
)

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
func (cpm *CompPatternMap) AddMapping(funcLabel string, hostname string, priority float64, overwrite bool) error {
	// only check for overwrite if asked to
	if !overwrite {
		host, present := cpm.FuncMap[funcLabel]
		if present && host != hostname {
			return fmt.Errorf("attempt to overwrite mapping of func label %s in %s",
				funcLabel, cpm.PatternName)
		}
	}

	// store the mapping
	priStr := strconv.FormatFloat(priority, 'f', -1, 64)
	cpm.FuncMap[funcLabel] = hostname + "," + priStr

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
			return fmt.Errorf("attempt to overwrite mapping of %s to comp pattern map dictionary", cpm.PatternName)
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
func (cpmd *CompPatternMapDict) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*cpmd)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*cpmd, "", "\t")
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
