package main

// exec-prb.go reads in descriptions of timings of funcs and devices, and
// creates a dictionary of timings that can be included into simulation runs

import (
	"encoding/csv"
	"fmt"
	"github.com/iti/cmdline"
	"github.com/iti/mrnes"
	cmptn "github.com/iti/mrnesbits"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// command line parameters are all about file and directory locations
// Even though we don't need the flags for the other MrNesbits structures we
// keep them here so that all the programs that build templates can use the same arguments file

// cmdlineParameters defines the variables we may see on the command line.  Most of these
// can be ignored and are included just so that a number of programs can be driven by the same command line
func cmdlineParameters() *cmdline.CmdParser {
	// create an argument parser
	cp := cmdline.NewCmdParser()
	cp.AddFlag(cmdline.StringFlag, "prbLib", false)      // directory of pre-builts
	cp.AddFlag(cmdline.StringFlag, "prbCSV", false)      // input table in CSV form
	cp.AddFlag(cmdline.StringFlag, "cpPrb", false)       // name of output file used for comp pattern templates
	cp.AddFlag(cmdline.StringFlag, "expPrb", false)      // name of output file used for expCfg templates
	cp.AddFlag(cmdline.StringFlag, "funcExecPrb", false) // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "devExecPrb", false)  // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "topoPrb", false)     // name of output file used for topo templates
	cp.AddFlag(cmdline.StringFlag, "CPInitPrb", false)   // name of output file used for func parameters templates

	return cp
}

// main is, well, the main entry point
func main() {
	// configure the command line parsing
	cp := cmdlineParameters()

	// now parse the command line, enabling queries of the states
	// of the variables that had been declared
	cp.Parse()

	// name of the pre-built template library
	prbLibDir := cp.GetVar("prbLib").(string)

	// name of the file with csv entries describing function and device timings
	csvFile := cp.GetVar("prbCSV").(string)

	// make sure this directory exists
	dirs := []string{prbLibDir}
	valid, err := cmptn.CheckDirectories(dirs)
	if !valid {
		panic(err)
	}

	// if csvFile is not empty, ensure that we can read it
	if len(csvFile) > 0 {
		// csvFile = filepath.Join(prbLibDir, csvFile)
		valid, err := cmptn.CheckReadableFiles([]string{csvFile})
		if !valid {
			panic(err)
		}
	}

	// check for access to template output files
	devExecPrbFile := cp.GetVar("devExecPrb").(string)

	// check for access to template output files
	funcExecPrbFile := cp.GetVar("funcExecPrb").(string)

	// join directory specifications with file name specifications
	//
	// path to file holding dictionary of lists of template files of experiment parameters
	funcExecPrbFile = filepath.Join(prbLibDir, funcExecPrbFile)

	// path to file holding dictionary of lists of template files of experiment parameters
	devExecPrbFile = filepath.Join(prbLibDir, devExecPrbFile)

	// check files to be created
	files := []string{funcExecPrbFile, devExecPrbFile}
	rdFiles := []string{}

	for _, file := range files {
		if len(file) > 0 {
			rdFiles = append(rdFiles, file)
		}
	}
	valid, err = cmptn.CheckOutputFiles(rdFiles)
	if !valid {
		panic(err)
	}

	// create the dictionaries to be populated

	// build a table of execution parameters for functions
	fel := cmptn.CreateFuncExecList("Experiment-1")

	// build a table of device timing parameters
	del := mrnes.CreateDevExecList("Experiment-1")

	// add timings for the StatefulTest comp pattern
	fel.AddTiming("mark", "x86", 1000, 1e-4)
	fel.AddTiming("mark", "pentium", 1000, 2e-4)
	fel.AddTiming("mark", "M1", 1000, 2e-4)
	fel.AddTiming("mark", "M2", 1000, 2e-4)

	fel.AddTiming("testFlag", "x86", 1000, 1e-5)
	fel.AddTiming("testFlag", "pentium", 1000, 2e-5)
	fel.AddTiming("testFlag", "M1", 1000, 1e-5)
	fel.AddTiming("testFlag", "M2", 1000, 1e-5)

	fel.AddTiming("process", "x86", 1000, 1e-5)
	fel.AddTiming("process", "pentium", 1000, 2e-5)
	fel.AddTiming("process", "M1", 1000, 1e-5)
	fel.AddTiming("process", "M2", 1000, 1e-5)

	// if csv input is given, read in the CSV records
	if len(csvFile) > 0 {
		csvf, err := os.Open(csvFile)
		if err != nil {
			panic(err)
		}
		fileReader := csv.NewReader(csvf)
		tbl, _ := fileReader.ReadAll()

		// tbl[0] containing "###" means a comment to be skipped
		// otherwise == "func" or "dev" indicating how to interpret the rest of the line.
		// if "func" the columns are
		//  1: function type
		//	2: CPU
		//  3: PcktLen
		//  4: MsgLen
		//	5: Execution time

		// if "dev" the columns are
		//  1: device operation
		//  2: device model
		//  3: baseline execution time
		//
		for idx := 0; idx < len(tbl); idx++ {
			if strings.Contains(tbl[idx][0], "###") {
				continue
			}
			if tbl[idx][0] == "func" {
				funcType := tbl[idx][1]
				cpuType := tbl[idx][2]
				pcktLen, _ := strconv.Atoi(tbl[idx][3])
				execTime, _ := strconv.ParseFloat(tbl[idx][5], 64)
				fel.AddTiming(funcType, cpuType, pcktLen, execTime)
			} else if tbl[idx][0] == "dev" {
				opType := tbl[idx][1]
				model := tbl[idx][2]
				execTime, _ := strconv.ParseFloat(tbl[idx][3], 64)
				del.AddTiming(opType, model, execTime)
			}
		}
	}
	// can add more timings here to fel and del if desired

	fmt.Printf("Added FuncExecList %s and DevExecList %s ExecDictSet\n", fel.ListName, del.ListName)
	fel.WriteToFile(funcExecPrbFile)
	del.WriteToFile(devExecPrbFile)

	fmt.Println("Output files written!")

}
