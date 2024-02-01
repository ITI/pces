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
	expPrbDict := mrnes.CreateExpCfgDict("Topo-1")
	expCfg := mrnes.CreateExpCfg(expPrbDict.DictName)

	// experiment parameters are largely about architectural parameters
	// that impact performance. Define some defaults (which can be overwritten later)
	//
	expCfg.AddParameter("Interface", "wired", "latency", "10e-6")
	expCfg.AddParameter("Interface", "wired", "latency", "10e-6")
	expCfg.AddParameter("Interface", "*", "bandwidth", "1000")

	expCfg.AddParameter("Host", "*", "CPU", "x86")
	expCfg.AddParameter("Host", "encrypt-host", "CPU", "M1")
	expCfg.AddParameter("Host", "decrypt-host", "CPU", "pentium")
	expCfg.AddParameter("Host", "server-host", "CPU", "M1")

	expCfg.AddParameter("Network", "name%%transNet", "latency", "1e-3")

	fmt.Printf("Added ExpCfg %s to ExpCfgDict\n", expCfg.Name)
	expPrbDict.AddExpCfg(expCfg, false)
	expPrbDict.WriteToFile(expPrbFile)

	fmt.Println("Output files written!")
}
