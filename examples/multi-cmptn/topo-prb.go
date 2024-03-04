package main

// topo-prb.go builds a topology for a pre-built library for mrnes.  Each topology is given a name
// and is written into a dictionary for later retrieval

import (
	"fmt"
	"github.com/iti/cmdline"
	"github.com/iti/mrnes"
	"path/filepath"
)

// cmdlineParameters configures for recognition of command line variables
func cmdlineParameters() *cmdline.CmdParser {
	// create an argument parser
	cp := cmdline.NewCmdParser()
	cp.AddFlag(cmdline.StringFlag, "prbLib", false) // directory of tmplates

	cp.AddFlag(cmdline.StringFlag, "cpPrb", false)       // name of output file used for comp pattern templates
	cp.AddFlag(cmdline.StringFlag, "expPrb", false)      // name of output file used for expCfg templates
	cp.AddFlag(cmdline.StringFlag, "funcExecPrb", false) // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "devExecPrb", false)  // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "prbCSV", false)      // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "topoPrb", false)     // name of output file used for topo templates
	cp.AddFlag(cmdline.StringFlag, "CPInitPrb", false)   // name of output file used for func parameters templates

	return cp
}

func main() {
	// configure command line variable recognition
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

	// check for access to template output files
	topoPrbFile := cp.GetVar("topoPrb").(string)

	// join directory specifications with file name specifications
	//
	topoPrbFile = filepath.Join(prbLibDir, topoPrbFile) // path to file holding template files of experiment parameters

	// check files to be created
	files := []string{topoPrbFile}
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

	// create the dictionaries to be populated
	// build a topology dictionary
	topoPrbDict := mrnes.CreateTopoCfgDict("TopoCfg-1")

	// build a topology named 'LAN-WAN-LAN' with three networks, four hosts.
	tcf := mrnes.CreateTopoCfgFrame("LAN-WAN-LAN")

	// two wired networks, one wireless network
	srcNet := mrnes.CreateNetworkFrame("srcNet", "LAN", "wired")
	dstNet := mrnes.CreateNetworkFrame("dstNet", "LAN", "wireless")
	transNet := mrnes.CreateNetworkFrame("transitNet", "WAN", "wired")

	// connect them.  transNet connects to both, the others just to transNet
	// ConnectNetworks returns a router that implements the connection
	src_trans_rtr, err1 := mrnes.ConnectNetworks(srcNet, transNet, false)
	dst_trans_rtr, err2 := mrnes.ConnectNetworks(dstNet, transNet, false)
	aggErr := mrnes.ReportErrs([]error{err1,err2})
	if aggErr != nil {
		panic(aggErr)
	}

	// create a switch hub for srcNet and transNet, and a wireless router hub for dstNet
	srcSwitch := mrnes.CreateSwitchFrame("srcSwitch", "cisco")
	srcNet.IncludeDev(srcSwitch, "wired", true)
 
	transSwitch := mrnes.CreateSwitchFrame("transSwitch", "cisco")
	transNet.IncludeDev(srcSwitch, "wired", true)

	dstHub := mrnes.CreateRouterFrame("dstHub", "fortinet")
	dstNet.IncludeDev(dstHub, "wireless", true)

	// create hosts srcHost and reflectHost for srcNet
	srcHost := mrnes.CreateHostFrame("hostNetA1", "Dell")
	reflectHost := mrnes.CreateHostFrame("hostNetA2", "Apple")
	srcNet.IncludeDev(srcHost, "wired", true)
	srcNet.IncludeDev(reflectHost, "wired", true)

	// attach these to the srcNet hub switch
	mrnes.ConnectDevs(srcHost, srcSwitch, true, srcNet.Name )
	mrnes.ConnectDevs(reflectHost, srcSwitch, true, srcNet.Name )

	// create host srvrHost for transNet and attach it
	srvrHost := mrnes.CreateHostFrame("hostNetT1", "Dell")
	mrnes.ConnectDevs(transSwitch, srvrHost, true, transNet.Name)

	// create host XXYY for dstNet and attach it
	dstHost := mrnes.CreateHostFrame("hostNetB1", "Dell")
	dstHub.WirelessConnectTo(dstHost, dstNet.Name)

	// connect the to the routers. Notice here that both
	// the hub in srcNet and hub in transNet connect to the router
	// that connects srcNet and transNet, but that the dstNet hub
	// connects only to the router between dstNet and transNet
	mrnes.ConnectDevs(srcSwitch, src_trans_rtr, true, srcNet.Name)
	mrnes.ConnectDevs(transSwitch, src_trans_rtr, true, transNet.Name)
	mrnes.ConnectDevs(transSwitch, dst_trans_rtr, true, transNet.Name)
	mrnes.ConnectDevs(dstHub, dst_trans_rtr, true, dstNet.Name)

	// connect the two routers
	mrnes.ConnectDevs(src_trans_rtr, dst_trans_rtr, false, transNet.Name)

	// gather up the hosts, switches, and routers, and stick them
	// in the TopoCfgFrame
	tcf.AddNetwork(srcNet)
	tcf.AddNetwork(dstNet)
	tcf.AddNetwork(transNet)

	// fill in any missing parts needed for the topology description
	topoCfgerr := tcf.Consolidate()
	if topoCfgerr != nil {
		panic(topoCfgerr)
	}

	// turn the pointer-oriented data structures into a flat string-based
	// version for serialization, then save to file
	tc := tcf.Transform()

	fmt.Printf("Added TopoCfg %s to TopoCfgDict\n", tc.Name)
	topoPrbDict.AddTopoCfg(&tc, false)
	topoPrbDict.WriteToFile(topoPrbFile)

	fmt.Println("Output files written!")

}
