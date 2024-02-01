package main

// run a model to drive development of MrNesbits

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"github.com/iti/mrnes"
	mrnb "github.com/iti/mrnesbits"
	"github.com/iti/cmdline"
	"github.com/iti/rngstream"
)

// command line parameters are all about file and directory locations
func cmdlineParameters() *cmdline.CmdParser {
	// create an argument parser
	cp := cmdline.NewCmdParser()
	cp.AddFlag(cmdline.StringFlag, "exptLib", true)		// library of experiments	
	cp.AddFlag(cmdline.StringFlag, "expt", true)		// subdirectory of experiments in library to run

	cp.AddFlag(cmdline.StringFlag, "cpInput", true)		// path to input file of completed comp patterns	
	cp.AddFlag(cmdline.StringFlag, "cpInitInput", true)	// path to input file of completed comp patterns	
	cp.AddFlag(cmdline.StringFlag, "funcExecInput", true)	// path to input file of execution maps of func timings
	cp.AddFlag(cmdline.StringFlag, "devExecInput", true)	// path to input file of execution maps of device timings
	cp.AddFlag(cmdline.StringFlag, "mapInput", true)	// path to input file describing mapping of comp pattern functions to host
	cp.AddFlag(cmdline.StringFlag, "topoInput", true)	// path to input file describing completed topology
	cp.AddFlag(cmdline.StringFlag, "expInput", true)	// path to input file describing completed expCfg 
	cp.AddFlag(cmdline.BoolFlag, "qnetsim", false)		// flag indicating that network sim ought to be 'quick'
	cp.AddFlag(cmdline.FloatFlag, "stop", true)	// run the simulation until this time (in seconds)
	
	return cp
}

func main() {
	// configure the command line parameters
	cp := cmdlineParameters()
	cp.Parse()

	exptLib := cp.GetVar("exptLib").(string)
	exptDir := cp.GetVar("expt").(string)

	// check existence of experiment library directory
	inputDir := filepath.Dir(exptLib)	
    dirInfo, err1 := os.Stat(inputDir) 
	if os.IsNotExist(err1) || !dirInfo.IsDir() {
		fmt.Printf("experiment lib directory %s does not exist\n", inputDir)
		os.Exit(1)
	}

	// create path to experiment subdirectory
	exptPath := filepath.Join(exptLib, exptDir)

	// create experiment file names for checking, and a synomym map for reference
	var syn map[string]string = make(map[string]string)
	inputs := []string{"cpInput","cpInitInput","funcExecInput","devExecInput", "mapInput","topoInput","expInput"}

	useYAML := true
	inputFiles := []string{}
	errs := []error{}

	for _, filename := range inputs {
		if !cp.IsLoaded(filename) {
			errs = append(errs, fmt.Errorf("command flag %s not included on the command line\n", filename))
			continue
		}
		filenameVar := cp.GetVar(filename).(string)
		fpath := filepath.Join(exptPath,filenameVar)
		syn[filename] = fpath
		inputFiles = append(inputFiles, fpath)
        fExt := path.Ext(fpath)
        if (fExt != ".yaml") && (fExt != ".yml") && (fExt != ".YAML") { 
            useYAML = false
        }
	}

	err := mrnb.ReportErrs(errs)
	if err != nil {
		panic(err)
	}

	// if -qnetsim is set we use the 'skip over network devices' version of network simulation
	if cp.IsLoaded("qnetsim") {
		mrnes.QkNetSim = true
	}

	// validate that these are all readable
	ok, err := mrnb.CheckReadableFiles(inputFiles)
	if !ok {
		panic(err)
	}

    // if requested, set the rng seed
    if cp.IsLoaded("rngseed") {
        seed := cp.GetVar("rngseed").(int64)
        rngstream.SetRngStreamMasterSeed(uint64(seed))
    }

	// build the experiment.  First the network stuff
	mrnes.BuildExperimentNet(syn, useYAML)

	// now the computation patterns, where initial events were scheduled
	evtmgr, err := mrnb.BuildExperimentCP(syn, useYAML)
	if err != nil {
		panic(err)
	}

	termination := cp.GetVar("stop").(float64)
	evtmgr.Run(termination)
	fmt.Println("Done")
}
