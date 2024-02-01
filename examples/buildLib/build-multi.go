package main

// build a model to drive development of MrNesbits

import (
	"fmt"
	"github.com/iti/cmdline"
	"github.com/iti/mrnes"
	cmptn "github.com/iti/mrnesbits"
	"os"
	"path"
	"path/filepath"
)

var empty []byte = make([]byte, 0)

// command line parameters are all about file and directory locations
func cmdlineParameters() *cmdline.CmdParser {
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
	cp.AddFlag(cmdline.StringFlag, "cpInitOutput", false)   // path to output file of completed comp patterns
	cp.AddFlag(cmdline.StringFlag, "funcExecOutput", false) // path to output file of execution maps of function timings
	cp.AddFlag(cmdline.StringFlag, "devExecOutput", false)  // path to output file of execution maps of device timings
	cp.AddFlag(cmdline.StringFlag, "mapOutput", false)      // path to output file describing mapping of comp pattern functions to host
	cp.AddFlag(cmdline.StringFlag, "topoOutput", false)     // path to output file describing completed topology
	cp.AddFlag(cmdline.StringFlag, "expOutput", false)      // path to output file describing completed expCfg

	return cp
}

func main() {

	// configure command-line parsing
	cp := cmdlineParameters()

	cp.Parse()

	// strings for the compPattern template library, and experiment library
	prbLibDir := cp.GetVar("prbLib").(string)
	exptLibDir := cp.GetVar("exptInputLib").(string)

	// make sure these directories exist
	dirs := []string{prbLibDir, exptLibDir}
	valid, err := cmptn.CheckDirectories(dirs)
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
		cpOutputFile = "cp.yaml"
	}

	cpInitOutputFile := cp.GetVar("cpInitOutput").(string)
	if len(cpInitOutputFile) == 0 {
		cpInitOutputFile = "cpInit.yaml"
	}

	funcExecOutputFile := cp.GetVar("funcExecOutput").(string)
	if len(funcExecOutputFile) == 0 {
		funcExecOutputFile = "funcexec.yaml"
	}

	devExecOutputFile := cp.GetVar("devExecOutput").(string)
	if len(devExecOutputFile) == 0 {
		devExecOutputFile = "devexec.yaml"
	}

	expOutputFile := cp.GetVar("expOutput").(string)
	if len(expOutputFile) == 0 {
		expOutputFile = "exp.yaml"
	}

	mapOutputFile := cp.GetVar("mapOutput").(string)
	if len(mapOutputFile) == 0 {
		mapOutputFile = "map.yaml"
	}

	topoOutputFile := cp.GetVar("topoOutput").(string)
	if len(topoOutputFile) == 0 {
		topoOutputFile = "topo.yaml"
	}

	if len(cpPrbFile) > 0 {
		cpPrbFile = filepath.Join(prbLibDir, cpPrbFile) // path to file holding template files of comp patterns
	}

	if len(cpInitPrbFile) > 0 {
		cpInitPrbFile = filepath.Join(prbLibDir, cpInitPrbFile) // path to file holding template files of comp patterns
	}
	cpInitExt := path.Ext(cpInitPrbFile)
	cpInitPrbIsYAML := (cpInitExt == ".yaml" || cpInitExt == ".YAML" || cpInitExt == ".yml")

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

	cpInitExt = path.Ext(cpInitOutputFile)
	cpInitIsYAML := (cpInitExt == ".yaml" || cpInitExt == ".YAML" || cpInitExt == ".yml")

	funcExecOutputFile = filepath.Join(exptLibDir, funcExecOutputFile) // path to output file of completed execution maps of function timings
	devExecOutputFile = filepath.Join(exptLibDir, devExecOutputFile)   // path to output file of completed execution maps of device timings
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

	valid, err = cmptn.CheckReadableFiles(rdFiles)
	if !valid {
		panic(err)
	}

	outputFiles := []string{cpOutputFile, cpInitOutputFile, funcExecOutputFile, devExecOutputFile, mapOutputFile, topoOutputFile, expOutputFile}

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

	valid, err = cmptn.CheckOutputFiles(outputFiles)

	// read in templates that have been requested
	var cpPrbDict *cmptn.CompPatternDict = nil

	if len(cpPrbFile) > 0 {
		pathExt := path.Ext(cpPrbFile)
		isYAML := (pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML")
		cpPrbDict, err = cmptn.ReadCompPatternDict(cpPrbFile, isYAML, empty)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Read comp pattern file %s\n", cpPrbFile)
	}

	var cpInitPrbDict *cmptn.CPInitListDict = nil

	if len(cpInitPrbFile) > 0 {
		pathExt := path.Ext(cpInitPrbFile)
		isYAML := (pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML")
		cpInitPrbDict, err = cmptn.ReadCPInitListDict(cpInitPrbFile, isYAML, empty)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Read comp pattern init file %s\n", cpInitPrbFile)
	}

	var expPrbDict *cmptn.ExpCfgDict = nil

	if len(expPrbFile) > 0 {
		pathExt := path.Ext(expPrbFile)
		isYAML := pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML"
		expPrbDict, err = cmptn.ReadExpCfgDict(expPrbFile, isYAML, empty)
		if err != nil {
			panic(nil)
		}
		fmt.Printf("Read experimental parameters template file %s\n", expPrbFile)
	}

	var funcExecList *cmptn.FuncExecList = nil
	if len(funcExecPrbFile) > 0 {
		pathExt := path.Ext(funcExecPrbFile)
		isYAML := (pathExt == ".yaml" || pathExt == ".yml" || pathExt == ".YAML")
		funcExecList, err = cmptn.ReadFuncExecList(funcExecPrbFile, isYAML, empty)
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
		fmt.Printf("Read topology template file %s\n", topoPrbDict.DictName)
	}

	// pull from template files copies of dictionaries that can be modified before
	// being written out for the experiment.
	cmptnDict := cmptn.CreateCompPatternDict("evaluate", false)
	cpInitDict := cmptn.CreateCPInitListDict("evaluate", false)

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
	DT, dtFound := cpPrbDict.RecoverCompPattern("StatefulTest", "simple-stateful-test")
	// fill in a Name, key used to reference this comp pattern when reading in the model
	if !dtFound {
		fmt.Printf("RecoverCompPattern %s %s failed\n", "StatefulTest", "simple-stateful-test")
		panic("ouch")
	}
	DT.Name = "simple-stateful-test"

	if !dtFound {
		fmt.Println("could not recover comp pattern StatefulTest from template file")
		os.Exit(1)
	}

	// bring in initialization for DT
	DTCPInit, dtInit := cpInitPrbDict.RecoverCPInitList("StatefulTest", "simple-stateful-test")
	if !dtInit {
		fmt.Println("could not recover comp pattern initialization for simple-stateful-test from template file")
		os.Exit(1)
	}

	// put AES comp pattern and CP into output dictionaries
	cmptnDict.AddCompPattern(DT, false, false)
	cpInitDict.AddCPInitList(DTCPInit, false)

	// bring in random branch
	RndBranch, rndFound := cpPrbDict.RecoverCompPattern("RandomBranch", "simple-random-branch")
	RndBranch.Name = "simple-random-branch"

	if !rndFound {
		fmt.Println("could not recover comp pattern simple-random-chain from template file")
		os.Exit(1)
	}

	// bring in initialization for RndBranch
	RBCPInit, rndInit := cpInitPrbDict.RecoverCPInitList("RandomBranch", "simple-random-branch")
	if !rndInit {
		fmt.Println("could not recover comp pattern initialization for simple-random-branch from template file")
		os.Exit(1)
	}

	cmptnDict.AddCompPattern(RndBranch, false, false)
	cpInitDict.AddCPInitList(RBCPInit, false)

	// comp pattern, cp init, cp mapping, exper Cfg, topo

	// fill in a Name, key used to reference this comp pattern when reading in the model
	clntSrvr, csFound := cpPrbDict.RecoverCompPattern("SimpleWeb", "client-server")

	// fill in a Name
	clntSrvr.Name = "client-server"

	if !csFound {
		fmt.Println("could not recover comp pattern simple-AES-chain from template file")
		os.Exit(1)
	}

	// bring in initialization for client-server
	csCPInit, cpInit := cpInitPrbDict.RecoverCPInitList("SimpleWeb", "client-server")
	if !cpInit {
		fmt.Println("could not recover comp pattern initialization for client-server from template file")
		os.Exit(1)
	}
	// put AES comp pattern and CP into output dictionaries
	cmptnDict.AddCompPattern(clntSrvr, false, false)

	// for illustration...set the self-initiation cycle on the source of the client-server to be once every 8 seconds

	initParamStr, present := csCPInit.Params["client"]
	if present {
		clientParams, _ := cmptn.DecodeStaticParameters(initParamStr, cpInitPrbIsYAML)

		// find the empty source label
		for idx, resp := range clientParams.Response {
			if len(resp.InEdge.SrcLabel) == 0 {
				// set the period to 8.0.   Be sure to change clientParams.Response directly
				// because changes to resp aren't permanent
				clientParams.Response[idx].Period = 8.0
				break
			}
		}
		// reserialize.   Be sure to write directly into csCPInit.Params["client"]
		csCPInit.Params["client"], _ = clientParams.Serialize(cpInitIsYAML)
	} else {
		fmt.Println("unable to reset periodicity of client-server 'client' function")
	}

	cpInitDict.AddCPInitList(csCPInit, false)

	// write the comp pattern and initialization dictionaries out
	cmptnDict.WriteToFile(cpOutputFile)
	cpInitDict.WriteToFile(cpInitOutputFile)

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
	cmpMapDict := cmptn.CreateCompPatternMapDict("LAN-WAN-LAN")
	cmpMap := cmptn.CreateCompPatternMap("simple-AES-chain")

	// AES functions are src, encrypt, decrypt, dst
	// hosts are encrypt-host, decrypt-host
	cmpMap.AddMapping("src", "encrypt-host", false)
	cmpMap.AddMapping("encrypt", "reflect-host", false)
	cmpMap.AddMapping("decrypt", "decrypt-host", false)
	cmpMap.AddMapping("consumer", "decrypt-host", false)

	cmpMapDict.AddCompPatternMap(cmpMap, false)

	cmpMap = cmptn.CreateCompPatternMap("client-server")
	cmpMap.AddMapping("client", "encrypt-host", false)
	cmpMap.AddMapping("server", "server-host", false)

	// client-server functions are client, server
	// hosts are encrypt-host, web-server
	cmpMap = cmptn.CreateCompPatternMap("simple-random-branch")
	cmpMap.AddMapping("generate1", "encrypt-host", false)
	cmpMap.AddMapping("generate2", "decrypt-host", false)
	cmpMap.AddMapping("branch", "server-host", false)
	cmpMap.AddMapping("consumer1", "reflect-host", false)
	cmpMap.AddMapping("consumer2", "decrypt-host", false)

	cmpMapDict.AddCompPatternMap(cmpMap, false)

	// random branch functions are generate1, generate2, branch, consumer1, consumer2

	// write the mapping dictionary out
	cmpMapDict.WriteToFile(mapOutputFile)

	fmt.Println("Output files written!")
}
