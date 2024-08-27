package pces

import (
	"fmt"
	"github.com/iti/mrnes"
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

func CheckFormats(fullpathmap map[string]string) (bool, error) {
	var cp CompPatternDict
	var cpinit CPInitListDict
	var funcexec FuncExecList
	var srdcfg SharedCfgGroupList
	var m CompPatternMapDict

	// mrnes structs
	var devexec mrnes.DevExecList
	// TODO Check if the file is either a dict or just a cfg
	var exp mrnes.ExpCfg
	var topo mrnes.TopoCfg

	for _, n := range maps.Keys(fullpathmap) {
		var err error
		filepath := fullpathmap[n]
		f, err := os.Open(filepath)
		var reader io.Reader
		reader = f

		// Using Decode instead of Unmarshal is much stricter. https://pkg.go.dev/gopkg.in/yaml.v3#Decoder
		// Decode checks that:
		// - No unexpected fields exist (by enforcing that Decoder.KnownFields = true)
		// - The hierarchical structure is correct (according to the corresponding Go struct)
		// - All data types match what is expected for each field (according to the corresponding Go struct)
		dec := yaml.NewDecoder(reader)
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
			err = dec.Decode(&devexec)
		case "srdCfg":
			err = dec.Decode(&srdcfg)
		case "exp":
			// mrnes's problem
			err = dec.Decode(&exp)
		case "topo":
			// mrnes's problem
			err = dec.Decode(&topo)
		case "map":
			err = dec.Decode(&m)
		default:
			// Optional config files
			err = nil
		}
		if err != nil {
			fmt.Println(err)
			return false, err
		}
	}
	funcMap, funcClassMap := ValidateCP(&cp)
	ValidateCPExtEdges(&cp, funcMap)
	ValidateCPInit(&cpinit, funcMap)
	cpumodels := ValidateFuncExec(&funcexec)
	devmodels, endpoints := ValidateTopo(&topo, cpumodels)
	ValidateMap(&m, funcMap, endpoints)
	ValidateSrdCfg(&srdcfg, funcClassMap)

	ValidateDevExec(&devexec, devmodels)
	ValidateExp(&exp)

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
			ParsePanic("cp", "CPNAME", cpname, "cptype", v.CPType, "cpname should contain the cptype")
		}
		if cpname != v.Name {
			ParsePanic("cp", "CPNAME", cpname, "name", v.Name, "name should match CPNAME header")
		}
		var funcLabels []string
		// Validate Function Declarations
		for _, f := range v.Funcs {
			// Validate class (pces builtin)
			if !validFuncClass(f.Class) {
				ParsePanic("cp", "CPNAME", cpname, "func class", f.Class, "func class must be declared and supported by the pces code base as a recognized class")
			}
			// Check for duplicate labels
			if slices.Contains(funcLabels, f.Label) {
				ParsePanic("cp", "CPNAME", cpname, "func label", f.Label, "func labels must be unique for each CPTYPE")
			}
			funcLabels = append(funcLabels, f.Label)
			funcClassMap[f.Class] = append(funcClassMap[f.Class], []string{cpname, f.Label})
		}
		// Validate Edge Connections
		for _, e := range v.Edges {
			if !slices.Contains(funcLabels, e.SrcLabel) {
				ParsePanic("cp", "CPNAME", cpname, "edges srclabel", e.SrcLabel, "srclabel must a valid FUNCLABEL")
			}
			if !slices.Contains(funcLabels, e.DstLabel) {
				ParsePanic("cp", "CPNAME", cpname, "edges dstlabel", e.DstLabel, "dstlabel must a valid FUNCLABEL")
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
					ParsePanic("cp", "CPNAME", cpname, "extedges srccp", connection.SrcCP, "srccp should be the current CPNAME")
				}
				// dstcp
				if !slices.Contains(maps.Keys(dict.Patterns), connection.DstCP) {
					ParsePanic("cp", "CPNAME", cpname, "extedges dstcp", connection.DstCP, "dstcp should be a valid CPTYPE contained under 'patterns'")
				}
				// srclabel
				if !slices.Contains(funcMap[connection.SrcCP], connection.SrcLabel) {
					ParsePanic("cp", "CPNAME", cpname, "extedges srclabel", connection.SrcLabel, "srclabel must be a valid FUNCLABEL declared under 'srccp'")
				}
				// dstlabel
				if !slices.Contains(funcMap[connection.DstCP], connection.DstLabel) {
					ParsePanic("cp", "CPNAME", cpname, "extedges dstlabel", connection.DstLabel, "dstlabel must be a valid FUNCLABEL declared under 'dstcp'")
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
			ParsePanic("cpInit", "CPNAME", cpname, "cptype", v.CPType, "cpname should contain the cptype")
		}
		if cpname != v.Name {
			ParsePanic("cpInit", "CPNAME", cpname, "name", v.Name, "name should match CPNAME header")
		}
		// Validate function declarations in cfg
		for funcname, _ := range v.Cfg {
			// Validate that the FUNCNAME exists for the current CPNAME
			if !slices.Contains(funcMap[cpname], funcname) {
				ParsePanic("cpInit", "CPNAME", cpname, "funcname", funcname, "funcname must be a valid function for CPNAME")
			}
			// TODO validate SERIALCFG
		}
		// TODO validate msgtype, this may have to be tracked from cp.yaml too
	}
}

// ValidateMap iterates through the passed CompPatternMapDict to verify the data contents
func ValidateMap(dict *CompPatternMapDict, funcMap map[string][]string, endpoints []string) {
	for cpname, v := range dict.Map {
		if cpname != v.PatternName {
			ParsePanic("map", "CPNAME", cpname, "patternname", v.PatternName, "patternname should match CPNAME")
		}
		for funclabel, endptname := range v.FuncMap {
			// Validate that the FUNCLABEL exists for the current CPNAME
			if !slices.Contains(funcMap[cpname], funclabel) {
				ParsePanic("map", "CPNAME", cpname, "funcname", funclabel, "funcname must be a valid function for CPNAME")
			}
			// Validate ENDPTNAME from endpoints declared in topo.yaml
			if !slices.Contains(endpoints, endptname) {
				ParsePanic("map", "CPNAME", cpname, "endptname", endptname, "endptname must match a valid endpoint declared in topo.yaml")
			}
		}
	}
}

// ValidateFuncExec iterates through the passed FuncExecList to verify the data contents
// Returns []string cpumodels to check against those in the network in topo.yaml
func ValidateFuncExec(l *FuncExecList) []string {
	var cpumodels []string
	for timingcode, v := range l.Times {
		for _, feDesc := range v {
			// identifier
			if feDesc.Identifier != timingcode {
				ParsePanic("funcExec", "TIMINGCODE", timingcode, "identifier", feDesc.Identifier, "identifier should match the TIMINGCODE")
			}
			// feDesc.param has no restrictions
			cpumodels = append(cpumodels, feDesc.CPUModel)
		}
	}
	return cpumodels
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
					ParsePanic("srdCfg", "GROUP NAME", v.Name, "label", val.Label, "invalid cpname/funclabel present")
				}
			}
		}
		// TODO does the content of cfgstr need to be validated somehow?
	}
}

func ParsePanic(filename string, container string, containerValue string, field string, fieldValue string, message string) {
	str := fmt.Errorf("[%v file parse error] PANIC in %v '%v' for %v '%v'\n%v", filename, container, containerValue, field, fieldValue, message)
	panic(str)
}

// TODO Move these functions to a file under the mrnes project when done testing

// ValidateDevExec iterates through the passed mrnes.DevExecList to verify the data contents
func ValidateDevExec(l *mrnes.DevExecList, devmodels []string) {
	/*for _, v := range l.Times {
		for _, val := range v {
			// TODO DEVMODEL is the union of SWITCHMODEL and ROUTERMODEL from topo.yaml
		}
	}*/
}

// ValidateExp iterates through the passed mrnes.ExpCfg to verify the data contents
func ValidateExp(cfg *mrnes.ExpCfg) {
	_, _, _ = mrnes.GetExpParamDesc()
	for _, v := range cfg.Parameters {
		// Validate paramobj
		if !slices.Contains(mrnes.ExpParamObjs, v.ParamObj) {
			ParsePanic("exp", "Name", cfg.Name, "paramobj", v.ParamObj, "paramobj must be a valid ParamObj")
		}
		// Validate attributes
		for _, attr := range v.Attributes {
			if !slices.Contains(mrnes.ExpAttributes[v.ParamObj], attr.AttrbName) && attr.AttrbName != "device" {
				ParsePanic("exp", "Name", cfg.Name, "attribute", attr.AttrbName, "attribute must be present in mrnes.ExpAttributes")
			}
		}
		// Validate param
		if !slices.Contains(mrnes.ExpParams[v.ParamObj], v.Param) {
			ParsePanic("exp", "Name", cfg.Name, "param", v.Param, "param must be present in mrnes.ExpParams")
		}
		// Validation of value can be easily done if we figure out the 'device' error
		//err := mrnes.ValidateParameter(v.ParamObj, v.Attributes, v.Param)
		//if err != nil {
		//	panic("ruh roh")
		//}
	}
}

// ValidateTopo iterates through the passed mrnes.TopoCfg to verify the data contents
// Returns []string devmodels, which is the Union of switchmodels and routermodels
// Returns []string endpoints, which is a slice containing the names of all defined endpoints
func ValidateTopo(cfg *mrnes.TopoCfg, cpumodels []string) ([]string, []string) {
	// Collect data from the defined routers to check them against network definitions
	var routers [][]string
	var routermodels []string
	for _, r := range cfg.Routers {
		routermodels = append(routermodels, r.Model)
		for _, i := range r.Interfaces {
			routers = append(routers, []string{r.Name, i.MediaType, i.Faces})
			ValidateInterfaceDesc(&i, r.Name)
		}
	}
	// Collect data from the defined switches to check them against network definitions
	var switches [][]string
	var switchmodels []string
	for _, s := range cfg.Switches {
		switchmodels = append(switchmodels, s.Model)
		for _, i := range s.Interfaces {
			switches = append(switches, []string{s.Name, i.MediaType, i.Faces})
			ValidateInterfaceDesc(&i, s.Name)
		}
	}
	var devmodels []string
	devmodels = append(routermodels, switchmodels...)
	// Remove duplicates from devmodels, resulting in the Union of routermodels and switchmodels
	// (not that there should really be any)
	slices.Sort(devmodels)
	slices.Compact(devmodels)
	// Collect data from the defined endpoints to check them against network definitions
	var endpoints []string
	for _, e := range cfg.Endpts {
		endpoints = append(endpoints, e.Name)
		// Validate that the CPUMODEL is declared in FuncExec
		if !slices.Contains(cpumodels, e.Model) {
			ParsePanic("topo", "Endpoint Name", e.Name, "model", e.Model, "Endpoint model must be a valid CPUMODEL declared in FuncExec")
		}
		for _, i := range e.Interfaces {
			ValidateInterfaceDesc(&i, e.Name)
		}
	}

	// mrnes doesn't have an existing static list of network scales or media types
	netscales := []string{"LAN", "WAN", "T3", "T2", "T1", "GeneralNet"}
	mediatypes := []string{"wired", "wireless"}
	for _, n := range cfg.Networks {
		// Validate network.netscale
		if !slices.Contains(netscales, n.NetScale) {
			ParsePanic("topo", "Name", n.Name, "netscale", n.NetScale, "netscale must be either LAN, WAN, T3, T2, T1, or GeneralNet")
		}
		// Validate network.mediatype
		// We use strings.ToLower() because switch statements in mrnes/net.go will accept
		// either 'wired'/'Wired' and 'wireless'/'Wireless'
		if !slices.Contains(mediatypes, strings.ToLower(n.MediaType)) {
			ParsePanic("topo", "Name", n.Name, "mediatype", n.MediaType, "mediatype must be either 'wired' or 'wireless'")
		}
		// TODO check groups are defined in SrdCfg
		// Make sure that all routers under 'network/routers' have been defined under the general 'routers' label
		for _, r := range n.Routers {
			found := false
			for _, router := range routers {
				if len(router) == 0 {
					continue
				}
				if router[0] == r && router[1] == n.MediaType && router[2] == n.Name {
					found = true
					break
				}
			}
			if !found {
				ParsePanic("topo", "Network", n.Name, "Router name, router, and mediatype", "", "Router name, router, and mediatype don't match for a network")
			}
		}
		// Make sure that all switches under 'network/switches' have been defined under the general 'switches' label
		for _, s := range n.Switches {
			found := false
			for _, switc := range switches {
				if len(switc) == 0 {
					continue
				}
				if switc[0] == s && switc[1] == n.MediaType && switc[2] == n.Name {
					found = true
					break
				}
			}
			if !found {
				ParsePanic("topo", "Network", n.Name, "Switch name, switch, and mediatype", "", "Switch name, switch, and mediatype don't match for a network")
			}
		}
		// Make sure that all endpoints under 'network/endpts' have been defined under the general 'endpts' label
		for _, e := range n.Endpts {
			if !slices.Contains(endpoints, e) {
				ParsePanic("topo", "Network", n.Name, "endpoint", e, "network endpoint not declared under the general NetworkDesc endpts")
			}
		}
	}
	return devmodels, endpoints
}

// ValidateInterfaceDesc is a helper function to verify a passed mrnes.IntrfcDesc object
func ValidateInterfaceDesc(int *mrnes.IntrfcDesc, attachedDevice string) {
	// Exactly one of the values of keys cable, carry, and wireless attributes is non-empty
	if !((!(int.Cable == "") && (int.Carry == "") && (len(int.Wireless) == 0)) ||
		((int.Cable == "") && !(int.Carry == "") && (len(int.Wireless) == 0)) ||
		((int.Cable == "") && (int.Carry == "") && !(len(int.Wireless) == 0))) {
		ParsePanic("topo IntrfcDesc", "Name", int.Name, "cable, carry, wireless", "", "Exactly one of the values of keys cable, carry, and wireless attributes should be non-empty")
	}
	// devtype should be either "Switch", "Router", or "Endpt"
	if !slices.Contains([]string{"Switch", "Router", "Endpt"}, int.DevType) {
		ParsePanic("topo IntrfcDesc", "Name", int.Name, "devtype", int.DevType, "devtype should be either 'Switch', 'Router', or 'Endpt'")
	}
	// device should be the same name of its parent Router/Switch
	if int.Device != attachedDevice {
		ParsePanic("topo IntrfcDesc", "Name", int.Name, "device", int.Device, "device should match the name of its parent Router/Switch")
	}
}
