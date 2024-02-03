package main

// build a model to drive development of MrNesbits

import (
	"fmt"
	"github.com/iti/cmdline"
	"github.com/iti/mrnes"
	mrnb "github.com/iti/mrnesbits"
	"os"
	"path"
	"path/filepath"
)

var empty []byte = make([]byte, 0)

// command line parameters are all about file and directory locations
func gatherParameters() *cmdline.CmdParser {
	// create an argument parser
	cp := cmdline.NewCmdParser()
	cp.AddFlag(cmdline.StringFlag, "prbLib", false)       // directory of tmplates
	cp.AddFlag(cmdline.StringFlag, "exptInputLib", false) // directory of experiment input files

	cp.AddFlag(cmdline.StringFlag, "cpPrb", false)       // name of file used for comp pattern templates
	cp.AddFlag(cmdline.StringFlag, "cpInitPrb", false)   // name of file used for comp pattern initialization
	cp.AddFlag(cmdline.StringFlag, "expPrb", false)      // name of file used for expCfg templates
	cp.AddFlag(cmdline.StringFlag, "funcExecPrb", false) // name of file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "devExecPrb", false)  // name of file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "topoPrb", false)     // name of file used for topo templates

	cp.AddFlag(cmdline.StringFlag, "cpOutput", false)       // path to output file of completed comp patterns
	cp.AddFlag(cmdline.StringFlag, "cpInitOutput", false)   // path to output file of completed comp pattern initializations
	cp.AddFlag(cmdline.StringFlag, "cpStateOutput", false)  // path to output file of completed comp patterns state over-rides
	cp.AddFlag(cmdline.StringFlag, "funcExecOutput", false) // path to output file of execution maps of function timings
	cp.AddFlag(cmdline.StringFlag, "devExecOutput", false)  // path to output file of execution maps of device timings
	cp.AddFlag(cmdline.StringFlag, "mapOutput", false)      // path to output file describing mapping of comp pattern functions to host
	cp.AddFlag(cmdline.StringFlag, "topoOutput", false)     // path to output file describing completed topology
	cp.AddFlag(cmdline.StringFlag, "expOutput", false)      // path to output file describing completed expCfg

	return cp
}

func main() {
	// get information about output files
	cp := gatherParameters()
	cp.Parse()

	// strings for the compPattern template library, and experiment library
	prbLibDir := cp.GetVar("prbLib").(string)
	exptLibDir := cp.GetVar("exptInputLib").(string)

	// make sure these directories exist
	dirs := []string{prbLibDir, exptLibDir}
	valid, err := mrnb.CheckDirectories(dirs)
	if !valid {
		panic(err)
	}

	cpPrbFile := cp.GetVar("cpPrb").(string)
	cpInitPrbFile := cp.GetVar("cpInitPrb").(string)
	expPrbFile := cp.GetVar("expPrb").(string)
	funcExecPrbFile := cp.GetVar("funcExecPrb").(string)
	devExecPrbFile := cp.GetVar("devExecPrb").(string)
	topoPrbFile := cp.GetVar("topoPrb").(string)

	cpOutputFile := cp.GetVar("cpOutput").(string)
	if len(cpOutputFile) == 0 {
		cpOutputFile = "cp.json"
	}

	cpInitOutputFile := cp.GetVar("cpInitOutput").(string)
	if len(cpInitOutputFile) == 0 {
		cpInitOutputFile = "cpInit.json"
	}

	cpStateOutputFile := cp.GetVar("cpStateOutput").(string)
	if len(cpStateOutputFile) == 0 {
		cpStateOutputFile = "cpState.json"
	}

	funcExecOutputFile := cp.GetVar("funcExecOutput").(string)
	if len(funcExecOutputFile) == 0 {
		funcExecOutputFile = "funcExec.json"
	}

	devExecOutputFile := cp.GetVar("devExecOutput").(string)
	if len(devExecOutputFile) == 0 {
		devExecOutputFile = "devExec.json"
	}

	expOutputFile := cp.GetVar("expOutput").(string)
	if len(expOutputFile) == 0 {
		expOutputFile = "exp.json"
	}

	mapOutputFile := cp.GetVar("mapOutput").(string)
	if len(mapOutputFile) == 0 {
		mapOutputFile = "map.json"
	}

	topoOutputFile := cp.GetVar("topoOutput").(string)
	if len(topoOutputFile) == 0 {
		topoOutputFile = "topo.json"
	}

	if len(cpPrbFile) > 0 {
		cpPrbFile = filepath.Join(prbLibDir, cpPrbFile) // path to file holding template files of comp patterns
	}

	if len(cpInitPrbFile) > 0 {
		cpInitPrbFile = filepath.Join(prbLibDir, cpInitPrbFile) // path to file holding template files of comp patterns
	}
	cpInitExt := path.Ext(cpInitPrbFile)
	cpInitPrbIsYAML := (cpInitExt == ".yaml" || cpInitExt == ".YAML" || cpInitExt == ".yml")

	if cpInitPrbIsYAML {
		fmt.Println("Using YAML formatted input")
	} else {
		fmt.Println("Using JSON formatted input")
	}

	if len(expPrbFile) > 0 {
		expPrbFile = filepath.Join(prbLibDir, expPrbFile) // path to file holding template files of experiment parameters
	}

	if len(funcExecPrbFile) > 0 {
		funcExecPrbFile = filepath.Join(prbLibDir, funcExecPrbFile) // path to file holding template files of experiment parameters
	}

	if len(devExecPrbFile) > 0 {
		devExecPrbFile = filepath.Join(prbLibDir, devExecPrbFile) // path to file holding template files of experiment parameters
	}

	if len(topoPrbFile) > 0 {
		topoPrbFile = filepath.Join(prbLibDir, topoPrbFile) // path to file holding template files of experiment parameters
	}

	cpOutputFile = filepath.Join(exptLibDir, cpOutputFile)         // path to output file of comp patterns
	cpInitOutputFile = filepath.Join(exptLibDir, cpInitOutputFile) // path to output file of comp patterns
	cpStateOutputFile = filepath.Join(exptLibDir, cpStateOutputFile) // path to output file of comp patterns

	cpInitExt = path.Ext(cpInitOutputFile)
	cpInitIsYAML := (cpInitExt == ".yaml" || cpInitExt == ".YAML" || cpInitExt == ".yml")

	if cpInitIsYAML {
		fmt.Println("cp initiation is YAML formatted")
	} else {
		fmt.Println("cp initiation is JSON formatted")
	}

	cpStateExt := path.Ext(cpStateOutputFile)
	cpStateIsYAML := (cpStateExt == ".yaml" || cpStateExt == ".YAML" || cpStateExt == ".yml")

	if cpStateIsYAML {
		fmt.Printf("cp state is YAML formatted\n")
	} else {
		fmt.Printf("cp state is JSON formatted\n")
	}

	funcExecOutputFile = filepath.Join(exptLibDir, funcExecOutputFile) // path to output file of completed execution maps of timings
	devExecOutputFile = filepath.Join(exptLibDir, devExecOutputFile)   // path to output file of completed execution maps of timings
	mapOutputFile = filepath.Join(exptLibDir, mapOutputFile)           // path to output file of maps of comp pattern functions to hosts
	topoOutputFile = filepath.Join(exptLibDir, topoOutputFile)         // path to output file with final topology description
	expOutputFile = filepath.Join(exptLibDir, expOutputFile)           // path to output file of completed execution parameters

	// check files that are supposed to exist already
	files := []string{cpPrbFile, cpInitPrbFile, expPrbFile, funcExecPrbFile, devExecPrbFile, topoPrbFile}
	rdFiles := []string{}

	for _, file := range files {
		if len(file) > 0 {
			rdFiles = append(rdFiles, file)
		}
	}

	valid, err = mrnb.CheckReadableFiles(rdFiles)
	if !valid {
		panic(err)
	}

	outputFiles := []string{cpOutputFile, cpInitOutputFile, cpStateOutputFile, funcExecOutputFile, devExecOutputFile, 
		mapOutputFile, topoOutputFile, expOutputFile}

	// check for the uniqueness of each name
	for idx := 0; idx < len(outputFiles)-1; idx++ {
		fileIdx := outputFiles[idx]
		for jdx := idx + 1; jdx < len(outputFiles); jdx++ {
			if fileIdx == outputFiles[jdx] {
				err := fmt.Errorf("Output file %s appears more than once\n", fileIdx)
				panic(err)
			}
		}
	}

	valid, err = mrnb.CheckOutputFiles(outputFiles)

	// read in templates that have been requested
	var cpPrbDict *mrnb.CompPatternDict = nil

	if len(cpPrbFile) > 0 {
		pathExt := path.Ext(cpPrbFile)
		isYAML := (pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML")
		cpPrbDict, err = mrnb.ReadCompPatternDict(cpPrbFile, isYAML, empty)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Read comp pattern file %s\n", cpPrbFile)
	}

	var cpInitPrbDict *mrnb.CPInitListDict = nil

	if len(cpInitPrbFile) > 0 {
		pathExt := path.Ext(cpInitPrbFile)
		isYAML := (pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML")
		cpInitPrbDict, err = mrnb.ReadCPInitListDict(cpInitPrbFile, isYAML, empty)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Read comp pattern init file %s\n", cpInitPrbFile)
	}

	var expPrbDict *mrnes.ExpCfgDict = nil

	if len(expPrbFile) > 0 {
		pathExt := path.Ext(expPrbFile)
		isYAML := pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML"
		expPrbDict, err = mrnes.ReadExpCfgDict(expPrbFile, isYAML, empty)
		if err != nil {
			panic(nil)
		}
		fmt.Printf("Read experimental parameters template file %s\n", expPrbFile)
	}

	var funcExecList *mrnb.FuncExecList = nil
	if len(funcExecPrbFile) > 0 {
		pathExt := path.Ext(funcExecPrbFile)
		isYAML := (pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML")
		funcExecList, err = mrnb.ReadFuncExecList(funcExecPrbFile, isYAML, empty)
		if err != nil {
			panic(nil)
		}
		fmt.Printf("Read function execution timing template file %s\n", funcExecList.ListName)
	}

	var devExecList *mrnes.DevExecList = nil
	if len(devExecPrbFile) > 0 {
		pathExt := path.Ext(devExecPrbFile)
		isYAML := (pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML")
		devExecList, err = mrnes.ReadDevExecList(devExecPrbFile, isYAML, empty)
		if err != nil {
			panic(nil)
		}
		fmt.Printf("Read device op execution timing template file %s\n", devExecList.ListName)
	}

	var topoPrbDict *mrnes.TopoCfgDict = nil

	if len(topoPrbFile) > 0 {
		pathExt := path.Ext(topoPrbFile)
		isYAML := pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML"
		topoPrbDict, err = mrnes.ReadTopoCfgDict(topoPrbFile, isYAML, empty)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Read topology template file %s from %s\n", topoPrbDict.DictName, topoPrbFile)
	}

	// pull from template files copies of dictionaries that can be modified before
	// being written out for the experiment.
	cmptnDict := mrnb.CreateCompPatternDict("evaluate", false)
	cpInitDict := mrnb.CreateCPInitListDict("evaluate", false)
	cpFuncStateDict := mrnb.CreateCPFuncStateDict("evaluate")

	// bring in AES chain
	AESChain, cpFound := cpPrbDict.RecoverCompPattern("AESChain", "simple-AES-chain")
	// fill in a Name, key used to reference this comp pattern when reading in the model
	AESChain.Name = "simple-AES-chain"

	if !cpFound {
		fmt.Println("could not recover comp pattern simple-AES-chain from template file")
		os.Exit(1)
	}

	// bring in initialization for AES
	AESCPInit, cpInit := cpInitPrbDict.RecoverCPInitList("AESChain", "simple-AES-chain")
	if !cpInit {
		fmt.Println("could not recover comp pattern initialization for simple-stateful-test from template file")
		os.Exit(1)
	}

	// put AES comp pattern and CP into output dictionaries
	cmptnDict.AddCompPattern(AESChain, false, false)
	cpInitDict.AddCPInitList(AESCPInit, false)

    
	// bring in StatefulTest 
	StatefulTest, scpFound := cpPrbDict.RecoverCompPattern("StatefulTest", "simple-bit-test")
	// fill in a Name, key used to reference this comp pattern when reading in the model
	StatefulTest.Name = "simple-bit-test"

	if !scpFound {
		fmt.Println("could not recover comp pattern simple-bit-test from template file")
		os.Exit(1)
	}

	// bring in initialization for StatefulTest
	STCPInit, scpInit := cpInitPrbDict.RecoverCPInitList("StatefulTest", "simple-bit-test")
	if !scpInit {
		fmt.Println("could not recover comp pattern initialization for simple-bit-test from template file")
		os.Exit(1)
	}

    // modify initial states for StatefulTest functions
    for funcName, paramString := range STCPInit.Params {
        switch funcName {
        case "src" :
            // branch is stateful, set "failperiod" to 1000
            param, err := mrnb.DecodeStatefulParameters(paramString, true)
            if err != nil {
                panic(err)
            }
            param.State["failperiod"] = "999"
            paramString, err = param.Serialize(true)
            if err != nil {
                panic(err)
            }
            STCPInit.Params[funcName] = paramString
        default :
        }
    }

    // bring in the state block for this
	stcpfs, scpState := STCPInit.RecoverCPFuncState("simple-bit-test")
	if !scpState {
		fmt.Println("could not recover comp pattern state for simple-bit-test from template file")
	}

	// put Stateful comp pattern, CP init, and CP state into output dictionaries
	cmptnDict.AddCompPattern(StatefulTest, false, false)
	cpInitDict.AddCPInitList(STCPInit, false)
    if scpState {
        cpFuncStateDict.AddCPFuncState(stcpfs)
    }

	// write the comp pattern and initialization dictionaries out
	cmptnDict.WriteToFile(cpOutputFile)
	cpInitDict.WriteToFile(cpInitOutputFile)
    cpFuncStateDict.WriteToFile(cpStateOutputFile)
    
	// pull in the experiment configuration, choosing Cfg number 1
	expCfg, xcpresent := expPrbDict.RecoverExpCfg("Topo-1")
	if !xcpresent {
		err := fmt.Errorf("Library ExpCfg dictionary Topo-1 not found")
		panic(err)
	}

	// write it out as is
	expCfg.WriteToFile(expOutputFile)

	funcExecList.WriteToFile(funcExecOutputFile)
	devExecList.WriteToFile(devExecOutputFile)

	// get a topology configuration
	topoCfg, tpresent := topoPrbDict.RecoverTopoCfg("LAN-WAN-LAN")
	if !tpresent {
		err := fmt.Errorf("Library topo dictionary LAN-WAN-LAN not found")
		panic(err)
	}

	// create an output topology configuration
	outTopoCfg := *topoCfg

	// write it out to file
	outTopoCfg.WriteToFile(topoOutputFile)

	// create a dictionary of mapping of Funcs to hosts
	cmpMapDict := mrnb.CreateCompPatternMapDict("LAN-WAN-LAN")
	cmpMap := mrnb.CreateCompPatternMap("simple-AES-chain")

	// AES functions are src, encrypt, decrypt, dst
	// hosts are hostNetA1, hostNetA2, hostNetB1, hostNetT1
	cmpMap.AddMapping("src", "hostNetA1", false)
	cmpMap.AddMapping("encrypt", "hostNetA2", false)
	cmpMap.AddMapping("decrypt", "hostNetB1", false)
	cmpMap.AddMapping("sink", "hostNetB1", false)
	cmpMapDict.AddCompPatternMap(cmpMap, false)

	// StatefulTest functions are src, branch, consumer1, consumer2
	// same set of hosts
	cmpMap = mrnb.CreateCompPatternMap("simple-bit-test")
	cmpMap.AddMapping("src", "hostNetA1", false)
	cmpMap.AddMapping("branch", "hostNetA2", false)
	cmpMap.AddMapping("consumer1", "hostNetB1", false)
	cmpMap.AddMapping("consumer2", "hostNetT1", false)
	cmpMapDict.AddCompPatternMap(cmpMap, false)
	
	// write the mapping dictionary out
	cmpMapDict.WriteToFile(mapOutputFile)

	fmt.Println("Output files written!")
}
