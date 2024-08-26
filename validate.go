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
		fmt.Println(n, fullpathmap[n])

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
	ValidateMap(&m, funcMap)
	ValidateFuncExec(&funcexec)
	ValidateSrdCfg(&srdcfg, funcClassMap)

	ValidateDevExec(&devexec)
	ValidateExp(&exp)
	ValidateTopo(&topo)
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
				PrintParseError("funcExec", "TIMINGCODE", timingcode, "identifier", feDesc.Identifier)
				panic("identifier should match the TIMINGCODE")
			}
			// feDesc.param has no restrictions
			// feDesc.cpumodel has no restrictions (unless we have a database of every CPU ever)
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

// TODO Move these functions to a file under the mrnes project when done testing

// ValidateDevExec iterates through the passed mrnes.DevExecList to verify the data contents
func ValidateDevExec(l *mrnes.DevExecList) {
	/*for _, v := range l.Times {
		for _, val := range v {
			// TODO is there a way to validate val.Model? Otherwise there may be nothing to validate here
		}
	}*/
}

// ValidateExp iterates through the passed mrnes.ExpCfg to verify the data contents
func ValidateExp(cfg *mrnes.ExpCfg) {
	_, _, _ = mrnes.GetExpParamDesc()
	for _, v := range cfg.Parameters {
		// Validate paramobj
		if !slices.Contains(mrnes.ExpParamObjs, v.ParamObj) {
			PrintParseError("exp", "Name", cfg.Name, "paramobj", v.ParamObj)
			panic("paramobj must be a valid ParamObj")
		}
		// Validate attributes
		for _, attr := range v.Attributes {
			if !slices.Contains(mrnes.ExpAttributes[v.ParamObj], attr.AttrbName) && attr.AttrbName != "device" {
				PrintParseError("exp", "Name", cfg.Name, "attribute", attr.AttrbName)
				panic("attribute must be present in mrnes.ExpAttributes")
			}
		}
		// Validate param
		if !slices.Contains(mrnes.ExpParams[v.ParamObj], v.Param) {
			PrintParseError("exp", "Name", cfg.Name, "param", v.Param)
			panic("param must be present in mrnes.ExpParams")
		}
		// Validation of value cannot be easily done
	}
}

// ValidateTopo iterates through the passed mrnes.TopoCfg to verify the data contents
func ValidateTopo(cfg *mrnes.TopoCfg) {
	// Collect data from the defined routers to check them against network definitions
	routers := make([][]string, len(cfg.Routers))
	for _, r := range cfg.Routers {
		for _, i := range r.Interfaces {
			routers = append(routers, []string{r.Name, i.MediaType, i.Faces})
			ValidateInterfaceDesc(&i, r.Name)
		}
	}
	// Collect data from the defined switches to check them against network definitions
	switches := make([][]string, len(cfg.Switches))
	for _, s := range cfg.Switches {
		for _, i := range s.Interfaces {
			switches = append(switches, []string{s.Name, i.MediaType, i.Faces})
			ValidateInterfaceDesc(&i, s.Name)
		}
	}
	// Collect data from the defined endpoints to check them against network definitions
	endpoints := make([]string, len(cfg.Endpts)-1)
	for _, e := range cfg.Endpts {
		endpoints = append(endpoints, e.Name)
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
			PrintParseError("topo", "Name", n.Name, "netscale", n.NetScale)
			panic("netscale must be either LAN, WAN, T3, T2, T1, or GeneralNet")
		}
		// Validate network.mediatype
		// We use strings.ToLower() because switch statements in mrnes/net.go will accept
		// either 'wired'/'Wired' and 'wireless'/'Wireless'
		if !slices.Contains(mediatypes, strings.ToLower(n.MediaType)) {
			PrintParseError("topo", "Name", n.Name, "mediatype", n.MediaType)
			panic("mediatype must be either 'wired' or 'wireless'")
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
				PrintParseError("topo", "Network", n.Name, "Router name, router, and mediatype", "")
				panic("Router name, router, and mediatype don't match for a network")
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
				PrintParseError("topo", "Network", n.Name, "Switch name, switch, and mediatype", "")
				panic("Switch name, switch, and mediatype don't match for a network")
			}
		}
		// Make sure that all endpoints under 'network/endpts' have been defined under the general 'endpts' label
		for _, e := range n.Endpts {
			if !slices.Contains(endpoints, e) {
				PrintParseError("topo", "Network", n.Name, "endpoint", e)
				panic("network endpoint not declared under the general NetworkDesc endpts")
			}
		}
	}
}

// ValidateInterfaceDesc is a helper function to verify a passed mrnes.IntrfcDesc object
func ValidateInterfaceDesc(int *mrnes.IntrfcDesc, attachedDevice string) {
	// Exactly one of the values of keys cable, carry, and wireless attributes is non-empty
	if !((!(int.Cable == "") && (int.Carry == "") && (len(int.Wireless) == 0)) ||
		((int.Cable == "") && !(int.Carry == "") && (len(int.Wireless) == 0)) ||
		((int.Cable == "") && (int.Carry == "") && !(len(int.Wireless) == 0))) {
		PrintParseError("topo IntrfcDesc", "Name", int.Name, "cable, carry, wireless", "")
		panic("Exactly one of the values of keys cable, carry, and wireless attributes should be non-empty")
	}
	// devtype should be either "Switch", "Router", or "Endpt"
	if !slices.Contains([]string{"Switch", "Router", "Endpt"}, int.DevType) {
		PrintParseError("topo IntrfcDesc", "Name", int.Name, "devtype", int.DevType)
		panic("devtype should be either 'Switch', 'Router', or 'Endpt'")
	}
	// device should be the same name of its parent Router/Switch
	if int.Device != attachedDevice {
		PrintParseError("topo IntrfcDesc", "Name", int.Name, "device", int.Device)
		panic("device should match the name of its parent Router/Switch")
	}
}
