package pces

import (
	"fmt"
	"os"
	"errors"
	"math"
	"github.com/iti/cmdline"
	"github.com/iti/mrnes"
	"github.com/iti/evt/evtm"
	"github.com/iti/rngstream"
	"golang.org/x/exp/slices"
	"path/filepath"
)


// cmdlineParams defines the parameters recognized
// on the command line
func cmdlineParams() *cmdline.CmdParser {
	// command line parameters are all about file and directory locations.
	// Even though we don't need the flags for the other pces structures we
	// keep them here so that all the programs that build templates can use the same arguments file
	// create an argument parser
	cp := cmdline.NewCmdParser()
	cp.AddFlag(cmdline.StringFlag, "exprmnt", true)  // string name of experiment being run
	cp.AddFlag(cmdline.StringFlag, "inputLib", true) // directory where model parameters are read from
	cp.AddFlag(cmdline.StringFlag, "outputLib", true) // directory where measurements and traces are stored
	cp.AddFlag(cmdline.StringFlag, "cp", true)       //
	cp.AddFlag(cmdline.StringFlag, "cpInit", true)   //
	cp.AddFlag(cmdline.StringFlag, "funcExec", true) // name of input file holding descriptions of functional timings
	cp.AddFlag(cmdline.StringFlag, "devExec", true)  // name of input file holding descriptions of device timings
	cp.AddFlag(cmdline.StringFlag, "map", true)      // file with mapping of comp pattern functions to hosts
	cp.AddFlag(cmdline.StringFlag, "exp", true)      // name of file used for run-time experiment parameters
	cp.AddFlag(cmdline.StringFlag, "mdfy", false)    // name of file used to modify exp experiment parameters
	cp.AddFlag(cmdline.StringFlag, "topo", true)    // name of output file used for topo templates
	cp.AddFlag(cmdline.StringFlag, "experiments", true)  // name of input file describing experiment parameters
	cp.AddFlag(cmdline.StringFlag, "trace", false)   // path to output file of trace records
	cp.AddFlag(cmdline.IntFlag, "rngseed", false)    // RNG seed 
	cp.AddFlag(cmdline.FloatFlag, "stop", false)      // run the simulation until this time (in seconds)
	cp.AddFlag(cmdline.BoolFlag, "json", false)      // input/output files in YAML, or JSON
	cp.AddFlag(cmdline.StringFlag, "csv", true)      // name of file where measurements will be written
	cp.AddFlag(cmdline.BoolFlag, "verbose", false)  // measure output is terse
	cp.AddFlag(cmdline.StringFlag, "tunits", true)   // units used in reporting time
	cp.AddFlag(cmdline.BoolFlag, "container", false)  // name of file where measurements will be written
	return cp
}

var syn map[string]string = make(map[string]string)
var evtMgr *evtm.EventManager = evtm.New()
var useTrace bool
var termination float64
var traceFile string
var csvFile string
var cp *cmdline.CmdParser
var ExprmntName string
var TimeUnits string
var MsrVerbose bool
var ExprmntsFile string

var GlobalSeed int64 = 1234567

// ReadSimArgs defines the command line parameters expected, and reads them
func ReadSimArgs() (*cmdline.CmdParser, *evtm.EventManager) {

	// define the command line parameters
	cp = cmdlineParams()

	// parse the command line
	cp.Parse()

	ExprmntName = cp.GetVar("exprmnt").(string)

	// string for the input directory
	inputDir := cp.GetVar("inputLib").(string)
	outputDir := cp.GetVar("outputLib").(string)

	container := false
	if cp.IsLoaded("container") {
		container = true
	}

	// if container is set change inputDir and outputDir
	// to be /tmp/extern/input and /tmp/extern/output
	if container {
		// create /tmp/extern if needed
		inputDir = "/tmp/extern/input"
		outputDir = "/tmp/extern/output"

		if _, err := os.Stat("/tmp/extern"); os.IsNotExist(err) {
			err := os.Mkdir("/tmp/extern", 0755)
			if err != nil {
				panic(errors.New("unable to create /tmp/extern"))
			}
			os.Mkdir("/tmp/extern/input", 0755)
			os.Mkdir("/tmp/extern/output", 0755)
		}
		if _, err := os.Stat("/tmp/extern/output"); os.IsNotExist(err) {
			err := os.Mkdir("/tmp/extern", 0755)
			if err != nil {
				panic(errors.New("unable to create /tmp/extern/output"))
			}

		}	
	}

	// if there is no stop argument, termination is left at the largest possible time
	termination = math.MaxFloat64
	if cp.IsLoaded("stop") {
		termination = cp.GetVar("stop").(float64)
	}

	TimeUnits = cp.GetVar("tunits").(string)
	if !(TimeUnits == "sec" || TimeUnits == "musec" || TimeUnits == "msec" || TimeUnits == "nsec") {
		panic("Time units parameter must be in {sec, msec, musec, nsec}")
	}

	MsrVerbose = cp.GetVar("verbose").(bool)

	// make sure these directories exist
	dirs := []string{inputDir}
	valid, err := CheckDirectories(dirs)
	if !valid {
		panic(err)
	}

	// check for access to input files
	fullpathmap := make(map[string]string)
	inFiles := []string{"cp", "cpInit", "funcExec", "devExec", "exp", "mdfy", "topo", "map", "experiments"}
	optionalFiles := []string{"mdfy", "experiments"}

	fullpath := []string{}
	errs := []error{}
	for _, filename := range inFiles {
		if !cp.IsLoaded(filename) {
			if !slices.Contains(optionalFiles, filename) {
				errs = append(errs, fmt.Errorf("command flag %s not included on the command line", filename))
			}
			syn[filename] = ""
			continue
		}
		basefile := cp.GetVar(filename).(string)
		fullfile := filepath.Join(inputDir, basefile)
		fullpath = append(fullpath, fullfile)

		fullpathmap[filename] = fullfile
		syn[filename] = fullfile
	}

	ExprmntsFile = syn["experiments"]

	err = ReportErrs(errs)
	if err != nil {
		panic(err)
	}

	// validate that these are all readable
	ok, err := CheckReadableFiles(fullpath)
	if !ok {
		panic(err)
	}

	outputFiles := make([]string, 0)

	if cp.IsLoaded("trace") {
		useTrace = true
		traceFile = cp.GetVar("trace").(string)
		if !container {
			traceFile = filepath.Join(outputDir, traceFile)
		} else {
			baseFile := filepath.Base(traceFile)
			traceFile = filepath.Join("/tmp/extern/output", baseFile)
		}
		outputFiles = append(outputFiles, traceFile)
	} 


	
	if cp.IsLoaded("csv") {
		csvFile = cp.GetVar("csv").(string)
		if !container {
			csvFile = filepath.Join(outputDir, csvFile)
		} else {
			baseFile := filepath.Base(csvFile)
			csvFile = filepath.Join("/tmp/extern/output", baseFile)
		}
		outputFiles = append(outputFiles, csvFile)
	} 
	
	_, err = CheckOutputFiles(outputFiles)
	if err != nil {
		panic(err)
	}


	// if requested, set the rng seed
	if cp.IsLoaded("rngseed") {
		seed := cp.GetVar("rngseed").(int)
		rngstream.SetRngStreamMasterSeed(uint64(seed))
		GlobalSeed = int64(seed)
	}
	return cp, evtMgr
}


func RunExperiment(expCntrl evtm.EventHandlerFunction, expCmplt evtm.EventHandlerFunction) {

	TraceMgr = mrnes.CreateTraceManager(ExprmntName, useTrace)

	// build the experiment.  First the network stuff
	// start the id counter at 1 (value passed is incremented before use)
	mrnes.BuildExperimentNet(syn, true, 0, TraceMgr)

	// now get the computation patterns and initialization structures
	// cpd  *CompPatternDict
	// cpid *CPInitDict
	// fel  *FuncExecList
	// cpmd *CompPatternMapDict
	cpd, cpid, fel, cpmd := GetExperimentCPDicts(syn)

	err := ContinueBuildExperimentCP(cpd, cpid, fel, cpmd, syn, mrnes.NumIDs, TraceMgr, evtMgr)
	if err != nil {
		panic(err)
	}

	// call function expControl to find start functions and run them

	expCntrl(evtMgr, nil, nil) 
	evtMgr.Run(termination)

	// call function expComplete to complete the experiment, write out measurements
	expCmplt(evtMgr, &csvFile, &ExprmntName) 
}




