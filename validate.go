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
	funcMap, funcMethodMap := ValidateCP(&cp)
	ValidateCPEdges(&cp, funcMap, funcMethodMap)
	ValidateCPInit(&cpinit, funcMap)
	ValidateSrdCfg(&srdcfg, funcMethodMap)
	ValidateExp(&exp)
	cpumodels := ValidateFuncExec(&funcexec)
	devmodels := ValidateDevExec(&devexec)
	endpoints := ValidateTopo(&topo, cpumodels, devmodels)
	ValidateMap(&m, funcMap, endpoints)

	return true, nil
}

// ValidateCP iterates through the passed CompPatternDict to verify the data contents
// map[string][]string funcMap is a mapping from each CPNAME to its declared FUNCLABELs. This is needed to validate other files.
// map[string][][]string funcMethodMap tracks a FUNCLABEL and FUNCLASS for each CPNAME, so that edges can validate their methodcodes
func ValidateCP(dict *CompPatternDict) (map[string][]string, map[string][][]string) {
	funcMap := make(map[string][]string)         // Keys = cpname, Vals = [FUNCLABEL, ...]
	funcMethodMap := make(map[string][][]string) // Keys = cpname, Vals = [[FUNCLABEL, FUNCLASS], ...]
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
			funcMethodMap[cpname] = append(funcMethodMap[cpname], []string{f.Label, f.Class})
		}
		funcMap[cpname] = funcLabels
	}
	return funcMap, funcMethodMap
}

// ValidateCPEdges iterates through the passed CompPatternDict to verify the ExtEdges specifically
// This is part of the validation for the cp file, but requires a full funcMap to verify that the
// external edge connections are valid
func ValidateCPEdges(dict *CompPatternDict, funcMap map[string][]string, funcMethodMap map[string][][]string) {
	CreateClassMethods()
	for cpname, v := range dict.Patterns {
		// Validate Edge Connections
		for _, e := range v.Edges {
			// For internal edges, srcLabel and dstLabel should exist under the same cpname
			if !slices.Contains(funcMap[cpname], e.SrcLabel) {
				ParsePanic("cp", "CPNAME", cpname, "edges srclabel", e.SrcLabel, "srclabel must a valid FUNCLABEL")
			}
			if !slices.Contains(funcMap[cpname], e.DstLabel) {
				ParsePanic("cp", "CPNAME", cpname, "edges dstlabel", e.DstLabel, "dstlabel must a valid FUNCLABEL")
			}

			// Find the class label for the current cpname and dstLabel
			var classLabel string
			for _, pair := range funcMethodMap[cpname] {
				if pair[0] == e.DstLabel {
					classLabel = pair[1]
					break
				}
			}
			// Check if the given methodcode is valid for the dstLabel's assigned class
			if !slices.Contains(maps.Keys(ClassMethods[classLabel]), e.MethodCode) {
				ParsePanic("cp", "CPNAME", cpname, "edges methodcode", e.MethodCode, "methodcode must be defined in the destination function")
			}
		}
		// Validate EXTEdges
		for _, ee := range v.ExtEdges {
			for _, connection := range ee {
				// Check that srccp and dstcp are valid CPNAMEs
				// srccp
				if connection.SrcCP != cpname {
					ParsePanic("cp", "CPNAME", cpname, "extedges srccp", connection.SrcCP, "srccp should be the current CPNAME")
				}
				// dstcp
				if !slices.Contains(maps.Keys(dict.Patterns), connection.DstCP) {
					ParsePanic("cp", "CPNAME", cpname, "extedges dstcp", connection.DstCP, "dstcp should be a valid CPTYPE contained under 'patterns'")
				}

				// Check that srclabel and dstlabel exist under their respective CPNAMEs
				// srclabel
				if !slices.Contains(funcMap[connection.SrcCP], connection.SrcLabel) {
					ParsePanic("cp", "CPNAME", cpname, "extedges srclabel", connection.SrcLabel, "srclabel must be a valid FUNCLABEL declared under 'srccp'")
				}
				// dstlabel
				if !slices.Contains(funcMap[connection.DstCP], connection.DstLabel) {
					ParsePanic("cp", "CPNAME", cpname, "extedges dstlabel", connection.DstLabel, "dstlabel must be a valid FUNCLABEL declared under 'dstcp'")
				}

				// Find the class label for the current cpname and srcLabel
				var classLabel string
				for _, pair := range funcMethodMap[cpname] {
					if pair[0] == connection.SrcLabel {
						classLabel = pair[1]
						break
					}
				}
				// Check if the given methodcode is valid for the srcLabel's assigned class
				if !slices.Contains(maps.Keys(ClassMethods[classLabel]), connection.MethodCode) {
					ParsePanic("cp", "CPNAME", cpname, "extedges methodcode", connection.MethodCode, "methodcode must be defined in the source function")
				}
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
		for funcname, serialcfg := range v.Cfg {
			// Validate that the expected FUNCNAME exists for the current CPNAME
			if !slices.Contains(funcMap[cpname], funcname) {
				ParsePanic("cpInit", "CPNAME", cpname, "funcname", funcname, "funcname must be a valid function for CPNAME")
			}

			// Validate SERIALCFG
			var reader = io.Reader(strings.NewReader(serialcfg))
			dec := yaml.NewDecoder(reader)
			var err error
			// Validate builtin functions connSrc, processPckt, cycleDst, and finish
			if slices.Contains(maps.Keys(FuncClassNames), funcname) {
				dec.KnownFields(true)

				switch funcname {
				case "connSrc":
					var cs connSrcCfg
					err = dec.Decode(&cs)
				case "processPckt":
					var pp processPcktCfg
					err = dec.Decode(&pp)
				case "cycleDst":
					var cd cycleDstCfg
					err = dec.Decode(&cd)
				case "finish":
					var f finishCfg
					err = dec.Decode(&f)
				}
				if err != nil {
					ParsePanic("cpinit", "CPNAME", cpname, "funcname", funcname, "invalid function config")
				}
			} else {
				// Partial validation for custom functions, doesn't enforce known fields and uses the broad "CmpPtnFuncInst"
				dec.KnownFields(false)

				var c CmpPtnFuncInst
				err = dec.Decode(&c)
				if err != nil {
					ParsePanic("cpinit", "CPNAME", cpname, "funcname", funcname, "invalid function config")
				}
			}
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
			// Validate that the expected FUNCLABEL exists for the current CPNAME
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
func ValidateSrdCfg(l *SharedCfgGroupList, funcMethodMap map[string][][]string) {
	// Loop through SharedStateGroupList's
	for _, v := range l.Groups {
		// Loop through GlobalFuncInstId's
		for _, val := range v.Instances {
			// Check if a CPNAME/FUNCLABEL pair is present under a FUNCLASS
			compareSlice := []string{val.Label, v.Class}
			// Element-wise check that funcMethodMap[CPNAME] matches the expected compareSlice
			for i, s := range funcMethodMap[val.CmpPtnName] {
				if s[i] != compareSlice[i] {
					ParsePanic("srdCfg", "GROUP NAME", v.Name, "label", val.Label, "invalid cpname/funclabel present")
				}
			}
		}
		// TODO This needs to be tested still, making sure FuncClasses is registered before this runs
		fc := FuncClasses[v.Class]
		var reader = io.Reader(strings.NewReader(v.CfgStr))
		var c CmpPtnFuncInst
		var err error

		dec := yaml.NewDecoder(reader)
		err = dec.Decode(&c)
		err = fc.ValidateCfg(&c)
		if err != nil {
			ParsePanic("srdCfg", "GROUP NAME", v.Name, "cfgstr", v.CfgStr, "invalid cfgstr")
		}
	}
}

// ParsePanic is a helper function for printing out formatted error messages
func ParsePanic(filename string, container string, containerValue string, field string, fieldValue string, message string) {
	str := fmt.Errorf("[%v file parse error] PANIC in %v '%v' for %v '%v'\n%v", filename, container, containerValue, field, fieldValue, message)
	panic(str)
}

// TODO Move these functions to a file under the mrnes project when done testing

// ValidateDevExec iterates through the passed mrnes.DevExecList to verify the data contents
// Return []string devmodels, which contains all router and switch models to check against topo.yaml
func ValidateDevExec(l *mrnes.DevExecList) []string {
	var devmodels []string
	for _, v := range l.Times {
		for _, val := range v {
			// DEVMODEL is the union of all SWITCHMODEL and ROUTERMODEL to validate topo.yaml
			devmodels = append(devmodels, val.Model)
		}
	}
	return devmodels
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
// Returns []string endpoints, which is a slice containing the names of all defined endpoints
func ValidateTopo(cfg *mrnes.TopoCfg, cpumodels []string, devmodels []string) []string {
	// Collect data from the defined routers to check them against network definitions
	var routers [][]string
	for _, r := range cfg.Routers {
		// Validate ROUTERMODEL
		if !slices.Contains(devmodels, r.Model) {
			ParsePanic("topo", "Router", r.Name, "model", r.Model, "Router model must be a valid ROUTERMODEL declared in topo.yaml")
		}
		for _, i := range r.Interfaces {
			routers = append(routers, []string{r.Name, i.MediaType, i.Faces})
			ValidateInterfaceDesc(&i, r.Name)
		}
	}
	// Collect data from the defined switches to check them against network definitions
	var switches [][]string
	for _, s := range cfg.Switches {
		// Validate SWITCHMODEL
		if !slices.Contains(devmodels, s.Model) {
			ParsePanic("topo", "Switch", s.Name, "model", s.Model, "Switch model must be a valid SWITCHMODEL declared in topo.yaml")
		}
		for _, i := range s.Interfaces {
			switches = append(switches, []string{s.Name, i.MediaType, i.Faces})
			ValidateInterfaceDesc(&i, s.Name)
		}
	}
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
	return endpoints
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
