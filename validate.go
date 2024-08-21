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

func CheckFormats(fullpathmap map[string]string) (bool, error) {
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
			var cp CompPatternDict
			err = dec.Decode(&cp)
			ValidateCP(&cp)
		case "cpInit":
			var cpinit CPInitListDict
			err = dec.Decode(&cpinit)
		case "funcExec":
			var funcexec FuncExecList
			err = dec.Decode(&funcexec)
		case "devExec":
			// mrnes's problem
		case "srdCfg":
			var srdcfg SharedCfgGroupList
			err = dec.Decode(&srdcfg)
		case "exp":
			// mrnes's problem
		case "topo":
			// mrnes's problem
		case "map":
			var m CompPatternMapDict
			err = dec.Decode(&m)
		default:
			// Optional config files
			err = nil
		}
		if err != nil {
			panic(err)
		}
	}
	return true, nil
}

func ValidateCP(dict *CompPatternDict) {

	for k, v := range dict.Patterns {
		//fmt.Println(k, v)
		cpname := k
		cptype := v.CPType
		if !strings.Contains(cpname, cptype) {
			panic("cpname should contain the cptype")
		}
		if cpname != v.Name {
			panic("name should match CPNAME header")
		}
		var funcLabels []string
		// Validate Function Declarations
		for _, f := range v.Funcs {
			// TODO check if class is valid according to the "Func Classes" section of documentation
			if slices.Contains(funcLabels, f.Label) {
				panic("Func labels must be unique for each CPTYPE")
			}
			funcLabels = append(funcLabels, f.Label)
		}
		// Validate Edge Connections
		for _, e := range v.Edges {
			if !slices.Contains(funcLabels, e.SrcLabel) || !slices.Contains(funcLabels, e.DstLabel) {
				panic("srclabel and dstlabel must be valid func labels")
			}
			// TODO check if methodcode is valid according to the "Method Codes" section of documentation
		}
		// Validate EXTEdges
		for _, ee := range v.ExtEdges {
			for _, connection := range ee {
				// srccp
				if connection.SrcCP != cpname {
					panic("srccp should be the current CPNAME")
				}
				// dstcp
				if !slices.Contains(maps.Keys(dict.Patterns), connection.DstCP) {
					panic("dstcp should be a valid CPTYPE contained under 'patterns'")
				}
				// srclabel
				if !slices.Contains(funcLabels, connection.SrcLabel) {
					panic("srclabel must be a valid FUNCLABEL declared under srccp")
				}
				// dstlabel
				// TODO in order to check dstlabel, we need to store the functions for every declared CPNAME in cp.yaml

				// msgtype has no restrictions listed in docs

				// TODO check if methodcode is valid according to the "Method Codes" section of documentation
			}
		}
	}
}
