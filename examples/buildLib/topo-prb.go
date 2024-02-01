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

	// make a broadcast domain for each network
	srcBcd, err3 := mrnes.CreateBroadcastDomainFrame(srcNet.Name, "srcBcd", "wired")
	dstBcd, err4 := mrnes.CreateBroadcastDomainFrame(dstNet.Name, "dstBcd", "wireless")
	srvrBcd, err5 := mrnes.CreateBroadcastDomainFrame(transNet.Name, "srvrBcd", "wired")
	aggErr := mrnes.ReportErrs([]error{err1, err2, err3, err4, err5})
	if aggErr != nil {
		panic(aggErr)
	}

	// connect the BCD hubs to the routers. Notice here that both
	// the hub in srcNet and hub in transNet connect to the router
	// that connects srcNet and transNet, but that the dstNet hub
	// connects only to the router between dstNet and transNet
	mrnes.ConnectDevs(srcBcd.SwitchHub, src_trans_rtr, srcNet.Name)
	mrnes.ConnectDevs(srvrBcd.SwitchHub, src_trans_rtr, transNet.Name)
	mrnes.ConnectDevs(dstBcd.RouterHub, dst_trans_rtr, dstNet.Name)

	// connect the two routers
	mrnes.ConnectDevs(src_trans_rtr, dst_trans_rtr, transNet.Name)

	// add hosts to broadcast domains
	srcHost := mrnes.CreateHostFrame("hostNetA1", "Host")
	srcBcd.AddHost(srcHost)

	reflectHost := mrnes.CreateHostFrame("hostNetA2", "Host")
	srcBcd.AddHost(reflectHost)

	dstHost := mrnes.CreateHostFrame("hostNetB1", "Host")
	dstBcd.AddHost(dstHost)

	srvrHost := mrnes.CreateHostFrame("hostNetT1", "Host")
	srvrBcd.AddHost(srvrHost)

	// add broadcast domains to networks
	srcNet.AddBrdcstDmn(srcBcd)
	dstNet.AddBrdcstDmn(dstBcd)
	transNet.AddBrdcstDmn(srvrBcd)

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
