package pces

import (
	"fmt"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"slices"
	"strings"
)

func CheckFileFormats(fullpathmap map[string]string) (bool, error) {
	return CheckFormats(fullpathmap)
}

// TODO add more detail into panic messages. e.g. what file threw the error and what CPNAME it failed on
func CheckFormats(fullpathmap map[string]string) (bool, error) {
	var cp CompPatternDict
	var cpinit CPInitListDict
	var funcexec FuncExecList
	var srdcfg SharedCfgGroupList
	var m CompPatternMapDict
	for _, n := range maps.Keys(fullpathmap) {
		var err error
		filepath := fullpathmap[n]
		f, err := os.Open(filepath)
		var r io.Reader
		r = f
		fmt.Println(n, fullpathmap[n])

		// Using Decode instead of Unmarshal is much stricter. https://pkg.go.dev/gopkg.in/yaml.v3#Decoder
		// Decode checks that:
		// - No unexpected fields exist (by enforcing that Decoder.KnownFields = true)
		// - The hierarchical structure is correct (according to the corresponding Go struct)
		// - All data types match what is expected for each field (according to the corresponding Go struct)
		dec := yaml.NewDecoder(r)
		dec.KnownFields(true)

		switch n {
		case "cp":
			err = dec.Decode(&cp)
		case "cpInit":
			err = dec.Decode(&cpinit)
		case "funcExec":
			err = dec.Decode(&funcexec)
		case "devExec":
			// mrnes's problem
		case "srdCfg":
			err = dec.Decode(&srdcfg)
		case "exp":
			// mrnes's problem
		case "topo":
			// mrnes's problem
		case "map":
			err = dec.Decode(&m)
		default:
			// Optional config files
			err = nil
		}
		if err != nil {
			return false, err
		}
	}
	funcMap, funcClassMap := ValidateCP(&cp)
	ValidateCPExtEdges(&cp, funcMap)
	ValidateCPInit(&cpinit, funcMap)
	ValidateMap(&m, funcMap)
	ValidateFuncExec(&funcexec)
	ValidateSrdCfg(&srdcfg, funcClassMap)
	return true, nil
}

// ValidateCP iterates through the passed CompPatternDict to verify the data contents
// Returns a mapping from each CPNAME to its declared Functions. This is needed to validate other files.
// The second mapping tracks the CPNAME and FUNCLABEL for each FUNCLASS, for the purposes of validating SrdCft
func ValidateCP(dict *CompPatternDict) (map[string][]string, map[string][][]string) {
	funcMap := make(map[string][]string)
	funcClassMap := make(map[string][][]string)
	for cpname, v := range dict.Patterns {
		if !strings.Contains(cpname, v.CPType) {
			PrintParseError("cp", "CPNAME", cpname, "cptype", v.CPType)
			panic("cpname should contain the cptype")
		}
		if cpname != v.Name {
			PrintParseError("cp", "CPNAME", cpname, "name", v.Name)
			panic("name should match CPNAME header")
		}
		var funcLabels []string
		// Validate Function Declarations
		for _, f := range v.Funcs {
			// TODO check if class is valid according to the "Func Classes" section of documentation
			if slices.Contains(funcLabels, f.Label) {
				PrintParseError("cp", "CPNAME", cpname, "func label", f.Label)
				panic("func labels must be unique for each CPTYPE")
			}
			funcLabels = append(funcLabels, f.Label)
			funcClassMap[f.Class] = append(funcClassMap[f.Class], []string{cpname, f.Label})
		}
		// Validate Edge Connections
		for _, e := range v.Edges {
			if !slices.Contains(funcLabels, e.SrcLabel) {
				PrintParseError("cp", "CPNAME", cpname, "edges srclabel", e.SrcLabel)
				panic("srclabel must a valid FUNCLABEL")
			}
			if !slices.Contains(funcLabels, e.DstLabel) {
				PrintParseError("cp", "CPNAME", cpname, "edges dstlabel", e.DstLabel)
				panic("dstlabel must a valid FUNCLABEL")
			}
			// TODO check if methodcode is valid according to the "Method Codes" section of documentation
		}
		funcMap[cpname] = funcLabels
	}
	return funcMap, funcClassMap
}

// ValidateCPExtEdges iterates through the passed CompPatternDict to verify the ExtEdges specifically
// This is part of the validation for the cp file, but requires a full funcMap to verify that the
// external edge connections are valid
func ValidateCPExtEdges(dict *CompPatternDict, funcMap map[string][]string) {
	for cpname, v := range dict.Patterns {
		// Validate EXTEdges
		for _, ee := range v.ExtEdges {
			for _, connection := range ee {
				// srccp
				if connection.SrcCP != cpname {
					PrintParseError("cp", "CPNAME", cpname, "extedges srccp", connection.SrcCP)
					panic("srccp should be the current CPNAME")
				}
				// dstcp
				if !slices.Contains(maps.Keys(dict.Patterns), connection.DstCP) {
					PrintParseError("cp", "CPNAME", cpname, "extedges dstcp", connection.DstCP)
					panic("dstcp should be a valid CPTYPE contained under 'patterns'")
				}
				// srclabel
				if !slices.Contains(funcMap[connection.SrcCP], connection.SrcLabel) {
					PrintParseError("cp", "CPNAME", cpname, "extedges srclabel", connection.SrcLabel)
					panic("srclabel must be a valid FUNCLABEL declared under 'srccp'")
				}
				// dstlabel
				if !slices.Contains(funcMap[connection.DstCP], connection.DstLabel) {
					PrintParseError("cp", "CPNAME", cpname, "extedges dstlabel", connection.DstLabel)
					panic("dstlabel must be a valid FUNCLABEL declared under 'dstcp'")
				}

				// msgtype has no restrictions listed in docs

				// TODO check if methodcode is valid according to the "Method Codes" section of documentation
			}
		}
	}
}

// ValidateCPInit iterates through the passed CPInitListDict to verify the data contents
func ValidateCPInit(dict *CPInitListDict, funcMap map[string][]string) {
	for cpname, v := range dict.InitList {
		if !strings.Contains(cpname, v.CPType) {
			PrintParseError("cpInit", "CPNAME", cpname, "cptype", v.CPType)
			panic("cpname should contain the cptype")
		}
		if cpname != v.Name {
			PrintParseError("cpInit", "CPNAME", cpname, "name", v.Name)
			panic("name should match CPNAME header")
		}
		// Validate function declarations in cfg
		for funcname, _ := range v.Cfg {
			// Validate that the FUNCNAME exists for the current CPNAME
			if !slices.Contains(funcMap[cpname], funcname) {
				PrintParseError("cpInit", "CPNAME", cpname, "funcname", funcname)
				panic("funcname must be a valid function for CPNAME")
			}
			// TODO validate SERIALCFG
		}
		// TODO validate msgtype, this may have to be tracked from cp.yaml too
	}
}

// ValidateMap iterates through the passed CompPatternMapDict to verify the data contents
func ValidateMap(dict *CompPatternMapDict, funcMap map[string][]string) {
	for cpname, v := range dict.Map {
		if cpname != v.PatternName {
			PrintParseError("map", "CPNAME", cpname, "patternname", v.PatternName)
			panic("patternname should match CPNAME")
		}
		for funcname, _ := range v.FuncMap {
			// Validate that the FUNCNAME exists for the current CPNAME
			if !slices.Contains(funcMap[cpname], funcname) {
				PrintParseError("map", "CPNAME", cpname, "funcname", funcname)
				panic("funcname must be a valid function for CPNAME")
			}
			// TODO validate ENDPTNAME
		}
	}
}

// ValidateFuncExec iterates through the passed FuncExecList to verify the data contents
func ValidateFuncExec(l *FuncExecList) {
	for timingcode, v := range l.Times {
		for _, feDesc := range v {
			// identifier
			if feDesc.Identifier != timingcode {
				PrintParseError("funcExec", "TIMECODE", timingcode, "identifier", feDesc.Identifier)
				panic("identifier should match the TIMINGCODE")
			}
			// feDesc.param has no restrictions
			// feDesc.cpumodel has no restrictions (unless we have a database of every CPU ever)
			// feDesc.pcktlen has no restrictions
			// feDesc.exectime has no restrictions
		}
	}
}

// ValidateSrdCfg iterates through the passed SharedCfgGroupList to verify the data contents
func ValidateSrdCfg(l *SharedCfgGroupList, funcClassMap map[string][][]string) {
	// Loop through SharedStateGroupList's
	for _, v := range l.Groups {
		// Loop through GlobalFuncInstId's
		for _, val := range v.Instances {
			// Check if a CPNAME/FUNCLABEL pair is present under a FUNCLASS
			compareSlice := []string{val.CmpPtnName, val.Label}
			for i, s := range funcClassMap[v.Class] {
				if s[i] != compareSlice[i] {
					PrintParseError("srdCfg", "GROUP NAME", v.Name, "label", val.Label)
					panic("CPNAME and FUNCLABEL not present under FUNCLASS")
				}
			}
		}
		// TODO does the content of cfgstr need to be validated somehow?
	}
}

func PrintParseError(filename string, container string, containerValue string, field string, fieldValue string) {
	fmt.Printf("[%v file parse error] PANIC in %v '%v' for %v '%v'\n", filename, container, containerValue, field, fieldValue)
}
