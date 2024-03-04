package main

// exp-prb.go creates groups of run-time default parameters
// and writes them to the pre-built library

import (
	"fmt"
	"github.com/iti/cmdline"
	"github.com/iti/mrnes"
	"path/filepath"
)

// cmdlineParameters define variables that may appear on the command line
func cmdlineParameters() *cmdline.CmdParser {
	// create an argument parser
	cp := cmdline.NewCmdParser()
	cp.AddFlag(cmdline.StringFlag, "prbLib", false)      // directory of tmplates
	cp.AddFlag(cmdline.StringFlag, "cpPrb", false)       // name of output file used for comp pattern templates
	cp.AddFlag(cmdline.StringFlag, "expPrb", false)      // name of output file used for expCfg templates
	cp.AddFlag(cmdline.StringFlag, "funcExecPrb", false) // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "devExecPrb", false)  // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "prbCSV", false)      // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "topoPrb", false)     // name of output file used for topo templates
	cp.AddFlag(cmdline.StringFlag, "CPInitPrb", false)   // name of output file used for func parameters templates

	return cp
}

// main is, well, main
func main() {
	// configure command line variables that will be recognized
	cp := cmdlineParameters()

	// parse the command line
	cp.Parse()

	// string for the template library
	prbLibDir := cp.GetVar("prbLib").(string)

	// make sure this directory exists
	dirs := []string{prbLibDir}
	valid, err := mrnes.CheckDirectories(dirs)
	if !valid {
		panic(err)
	}

	// check for access to template output file
	expPrbFile := cp.GetVar("expPrb").(string)

	// join directory specifications with file name specifications
	expPrbFile = filepath.Join(prbLibDir, expPrbFile) // path to file holding template files of experiment parameters

	// check files to be created
	files := []string{expPrbFile}
	rdFiles := []string{}

	for _, file := range files {
		if len(file) > 0 {
			rdFiles = append(rdFiles, file)
		}
	}

	valid, err = mrnes.CheckOutputFiles(rdFiles)
	if !valid {
		panic(err)
	}

	// create the dictionary to be populated
	expPrbDict := mrnes.CreateExpCfgDict("Topo")
	expCfg := mrnes.CreateExpCfg("Topo")

	// experiment parameters are largely about architectural parameters
	// that impact performance. Define some defaults (which can be overwritten later)
	//
	as := mrnes.AttrbStruct{}
	as.AttrbName = "media"
	as.AttrbValue = "wired"
	wiredAttrbs := []mrnes.AttrbStruct{as} 

	as = mrnes.AttrbStruct{}
	as.AttrbName = "media"
	as.AttrbValue = "wireless"
	wirelessAttrbs := []mrnes.AttrbStruct{as} 

	as = mrnes.AttrbStruct{}
	as.AttrbName = "*"
	as.AttrbValue = ""
	wcAttrbs := []mrnes.AttrbStruct{as} 

	// latency is time across attached cable
	err1 := expCfg.AddParameter("Interface", wiredAttrbs, "latency", "10e-4")
	err2 := expCfg.AddParameter("Interface", wirelessAttrbs, "latency", "10e-3")
	err3 := expCfg.AddParameter("Interface", wiredAttrbs, "bandwidth", "1000")
	// delay is time for a bit to transition interface
	err4 := expCfg.AddParameter("Interface", wcAttrbs, "delay", "1e-6")

	err5 := expCfg.AddParameter("Interface", wirelessAttrbs, "bandwidth", "500")
	err6 := expCfg.AddParameter("Host", wcAttrbs, "CPU", "x86")
	err7 := expCfg.AddParameter("Network", wiredAttrbs, "latency", "10e-3")
	err8 := expCfg.AddParameter("Network", wirelessAttrbs, "latency", "100e-3")
	aggErr := mrnes.ReportErrs([]error{err1, err2, err3, err4, err5, err6,err7,err8})
	if aggErr != nil {
		panic(aggErr)
	}
	fmt.Printf("Added ExpCfg for %s to ExpCfgDict in file %s\n", "Stateful", expPrbFile)
	expPrbDict.AddExpCfg(expCfg, false)
	expPrbDict.WriteToFile(expPrbFile)

	fmt.Println("Output files written!")
}
