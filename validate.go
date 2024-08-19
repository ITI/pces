package pces

import (
	"fmt"
	"golang.org/x/exp/maps"
)

func CheckFileFormats(fullpathmap map[string]string) (bool, error) {
	return CheckFormats(fullpathmap)
}

func CheckFormats(fullpathmap map[string]string) (bool, error) {
	empty := make([]byte, 0)
	for _, n := range maps.Keys(fullpathmap) {
		var err error
		filepath := fullpathmap[n]
		fmt.Println(n, fullpathmap[n])
		switch n {
		case "cp":
			var cp *CompPatternDict
			cp, err = ReadCompPatternDict(filepath, true, empty)
			cp = cp
		case "cpInit":
			var cpinit *CPInitListDict
			cpinit, err = ReadCPInitListDict(filepath, true, empty)
			cpinit = cpinit
		case "funcExec":
			var funcexec *FuncExecList
			funcexec, err = ReadFuncExecList(filepath, true, empty)
			funcexec = funcexec
		case "devExec":
			// mrnes's problem
		case "srdCfg":
			var srdcfg *SharedCfgGroupList
			srdcfg, err = ReadSharedCfgGroupList(filepath, true, empty)
			srdcfg = srdcfg
		case "exp":
			// mrnes's problem
		case "topo":
			// mrnes's problem
		case "map":
			var m *CompPatternMapDict
			m, err = ReadCompPatternMapDict(filepath, true, empty)
			m = m
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
