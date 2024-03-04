package main

// code to build template of MrNesbits CompPattern structures and their initialization structs.
// see adjacent README.md 
import (
	_ "fmt"
	"github.com/iti/cmdline"
	cmptn "github.com/iti/mrnesbits"
	"github.com/iti/mrnes"
	_ "path"
	"path/filepath"
	"strings"
	"strconv"
)

// cmdlineParams defines the parameters recognized
// on the command line
func cmdlineParams() *cmdline.CmdParser {
	// command line parameters are all about file and directory locations.
	// Even though we don't need the flags for the other MrNesbits structures we
	// keep them here so that all the programs that build templates can use the same arguments file
	// create an argument parser
	cp := cmdline.NewCmdParser()
	cp.AddFlag(cmdline.StringFlag, "outputLib", true)	 // directory where model is written
	cp.AddFlag(cmdline.StringFlag, "cp", true)		 // name of output file for computation pattern
	cp.AddFlag(cmdline.StringFlag, "cpInit", true)	 // name of output file for initialization blocks for CPs
	cp.AddFlag(cmdline.StringFlag, "cpState", true)	 //	name of output file where CP func state blocks are placed
	cp.AddFlag(cmdline.StringFlag, "funcExec", true)  // name of output file used for function timings
	cp.AddFlag(cmdline.StringFlag, "devExec", true)	 // name of output file used for device operations timings
	cp.AddFlag(cmdline.StringFlag, "map", true)		 // file with mapping of comp pattern functions to hosts
	cp.AddFlag(cmdline.StringFlag, "exp", true)		 // name of output file used for run-time experiment parameters
	cp.AddFlag(cmdline.IntFlag,	   "hosts", true)	 // number of hosts in topology
	cp.AddFlag(cmdline.StringFlag, "topo", false)    // name of output file used for topo templates

	return cp
}

// main gives the entry point
func main() {
	// define the command line parameters
	cp := cmdlineParams()

	// parse the command line
	cp.Parse()

	// string for the output directory
	outputLib := cp.GetVar("outputLib").(string)

	// make sure this directory exists
	dirs := []string{outputLib}
	valid, err := cmptn.CheckDirectories(dirs)
	if !valid {
		panic(err)
	}

	// check for access to output files
	fullpathmap := make(map[string]string)
	outFiles := []string{"cp", "cpInit", "cpState", "funcExec", "devExec", "exp", "topo", "map"}
	fullpath := []string{}
	for _, filename := range outFiles {
		basefile := cp.GetVar(filename).(string)
		fullfile := filepath.Join(outputLib, basefile)
		fullpath = append(fullpath, fullfile)
		fullpathmap[filename] = fullfile
	}
	
	valid, err = cmptn.CheckOutputFiles(fullpath)
	if !valid {
		panic(err)
	}

	// we can vary the number of hosts that are addressed
	numHosts := cp.GetVar("hosts").(int)

	// some of the outputs are stored in 'dictionaries' that have their own
	// name, and index contents by computational pattern.  The arguments
	// to CreateXXYYZZDict are dictionary names and are ignored withing mrnesbits
	cpDict := cmptn.CreateCompPatternDict("BITS-1", true)
	cpInitDict := cmptn.CreateCPInitListDict("BITS-1", true)
    cpFuncStateDict := cmptn.CreateCPFuncStateDict("BITS-1")

	// create a CompPattern that looks like the initial BITS architecture
	encryptPerf := cmptn.CreateCompPattern("encryptPerf")

	// 'src' will periodically generate a packet, destination of that embedded in the msg header
	src := cmptn.CreateFunc("generate", "src", "stateful")
	encryptPerf.AddFunc(src)

	// self-initiation message has type 'initiate', and also edge label 'initiate'
	encryptPerf.AddEdge(src.Label, src.Label, "initiate", "initiate")

	// the packet will be directed to the 'crypto' function
	crypto := cmptn.CreateFunc("select", "crypto", "stateful")
	encryptPerf.AddFunc(crypto)

	// src directs an edge to crypto, message type "data", edge label "crypto"
	encryptPerf.AddEdge(src.Label, crypto.Label, "data", "crypto")

	// crypto directs an edge to src, message type "data", edge label "src"
	encryptPerf.AddEdge(crypto.Label, src.Label, "data", "src")

	allhosts := []string{}
	allhostFuncs := []*cmptn.Func{}

	// the number of hosts was input on the command line
	for hostId:=0; hostId<numHosts; hostId += 1 {
		// create a name for the host of the form "host-x", x=0,1, ..., numHosts-1	
		strId := strconv.Itoa(hostId)
		hostName := "host-"+strId
		allhosts = append(allhosts, hostName)

		// every host has cost type "process" and execution type "static"
		hostFunc := cmptn.CreateFunc("process", hostName, "static")
		allhostFuncs = append(allhostFuncs, hostFunc)

		// include the host in the computation pattern
		encryptPerf.AddFunc(hostFunc)
	
		// crypto directs an edge to host-x, message type "data" and edge label "host-x"	
		encryptPerf.AddEdge(crypto.Label, hostFunc.Label, "data", hostName)
	
		// host-x directs an edge to crypto, message type "data" and edge label "crypto"
		encryptPerf.AddEdge(hostFunc.Label, crypto.Label, "data", "crypto")
	}

	// include the encryptPerf CompPattern in the library
	cpDict.AddCompPattern(encryptPerf, true, false)

	// create initialization structs for each of the encryptPerf CP functions.
	//
	// create the InEdges and OutEdges needed for the stateful function initiationization
	srcFromSrcEdge  := cmptn.InEdge{SrcLabel: src.Label, MsgType: "initiate"}
	srcToCryptoEdge := cmptn.OutEdge{DstLabel: crypto.Label, MsgType: "data"}
	srcFromCryptoEdge := cmptn.InEdge{SrcLabel: crypto.Label, MsgType: "data"}

	cryptoFromSrcEdge := cmptn.InEdge{SrcLabel: src.Label, MsgType: "data"}
	cryptoToSrcEdge := cmptn.OutEdge{DstLabel: src.Label, MsgType: "data"}

	cryptoToHostEdge   := make([]cmptn.OutEdge, numHosts)
	hostToCryptoEdge   := make([]cmptn.OutEdge, numHosts)

	cryptoFromHostEdge := make([]cmptn.InEdge, numHosts)
	hostFromCryptoEdge := make([]cmptn.InEdge, numHosts)

	for idx, host := range allhostFuncs {
		cryptoToHostEdge[idx]   = cmptn.OutEdge{DstLabel: host.Label, MsgType: "data"}
		cryptoFromHostEdge[idx] = cmptn.InEdge{SrcLabel: host.Label, MsgType: "data"}
		hostToCryptoEdge[idx]   = cmptn.OutEdge{DstLabel: crypto.Label, MsgType: "data"}
		hostFromCryptoEdge[idx] = cmptn.InEdge{SrcLabel: crypto.Label, MsgType: "data"}
	}	

	// create the Action for the Src Function

	// one action at src from self-initiation, another when a message comes back from crypto
	srcActions := make([]cmptn.ActionResp,2)

	// action from self-initiation
	srcFromSrcActionDesc := cmptn.ActionDesc{Select:"cycle", CostType:"generate"}
	srcFromSrcActionResp := cmptn.CreateActionResp(srcFromSrcEdge, srcFromSrcActionDesc, 4)

	// action from message return from crypto
	srcFromCryptoActionDesc := cmptn.ActionDesc{Select:"rrt", CostType:"rrt"}
	srcFromCryptoActionResp := cmptn.CreateActionResp(srcFromCryptoEdge, srcFromCryptoActionDesc, 0)

	srcActions[0] = srcFromSrcActionResp
	srcActions[1] = srcFromCryptoActionResp

	// create the Actions for the Crypto Function
	cryptoActions := make([]cmptn.ActionResp, numHosts+1)
	cryptoFromSrcDesc := cmptn.ActionDesc{Select:"decrypt-rsa", CostType:"decrypt-rsa"}
	cryptoFromSrcResp := cmptn.CreateActionResp(cryptoFromSrcEdge, cryptoFromSrcDesc, 0) 
	
	cryptoFromHostDesc := cmptn.ActionDesc{Select:"encrypt-rsa", CostType:"encrypt-rsa"}
	for idx := 0; idx< len(allhostFuncs); idx += 1 {
		cryptoActions[idx] = cmptn.CreateActionResp(cryptoFromHostEdge[idx], cryptoFromHostDesc, 0)
	}
	cryptoActions[numHosts] = cryptoFromSrcResp

	// we have the pieces now to put initializations together
	epCPInit := cmptn.CreateCPInitList("encryptPerf", "encryptPerf", true)

	// flesh out the messages. Each type has a packetlength of 1000 bytes and a message length of
	// 1500 bytes
	epInitMsg := cmptn.CreateCompPatternMsg("initiate", 1000, 1500)
	epDataMsg := cmptn.CreateCompPatternMsg("data", 1000, 1500)

	// remember these message specifications
	epCPInit.AddMsg(epInitMsg)
	epCPInit.AddMsg(epDataMsg)

	// state variables for src
	//	- "hosts" : comma-separated string of host names through which to cycle
	srcState := make(map[string]string)
	srcState["hosts"] = strings.Join(allhosts,",")
	srcParams := cmptn.CreateStatefulParameters(encryptPerf.CPType, src.Label, srcActions, srcState)

	// add the branching responses that may happen at src
	srcParams.AddResponse(srcFromSrcEdge, srcToCryptoEdge, "crypto", 5.0)
	srcParams.AddResponse(srcFromCryptoEdge, cmptn.EmptyOutEdge, "src", 0)

	// bundle it up
	srcParamStr, _ := srcParams.Serialize(true)
	epCPInit.AddParam(src.Label, src.ExecType, srcParamStr)


	// state variables for crypto
	cryptoState := make(map[string]string)

	// serialize initialization
	cryptoParams := cmptn.CreateStatefulParameters(encryptPerf.CPType, crypto.Label, cryptoActions, cryptoState)

	// add the branching responses that may happen at crypto
	for idx, hostFunc := range allhostFuncs {
		cryptoParams.AddResponse(cryptoFromSrcEdge, cryptoToHostEdge[idx], hostFunc.Label, 0)
		cryptoParams.AddResponse(cryptoFromHostEdge[idx], cryptoToSrcEdge, src.Label, 0)
	}
	cryptoParamStr, _ := cryptoParams.Serialize(true)
	epCPInit.AddParam(crypto.Label, crypto.ExecType, cryptoParamStr)

	// include this CompPattern's initialization information into the CP initialization dictionary
	cpInitDict.AddCPInitList(epCPInit, false)

	// hostFunc responses are all static
	for idx, hostFunc := range allhostFuncs {
		sparams := cmptn.CreateStaticParameters(encryptPerf.CPType, hostFunc.Label)
		sparams.AddResponse(hostFromCryptoEdge[idx], hostToCryptoEdge[idx], hostToCryptoEdge[idx].MsgType, 0.0)
		sparamStr, _ := sparams.Serialize(true)
		epCPInit.AddParam(hostFunc.Label, hostFunc.ExecType, sparamStr)
	}
    fel := cmptn.CreateFuncExecList("BITS-1")
	fel.AddTiming("process","x86", 1000, 5e-2)
	fel.AddTiming("process","M1",  1000, 2e-2)
	fel.AddTiming("process","M2",  1000, 5e-3)

	// include the timings for the operations just specified 
	fel.AddTiming("generate", "x86", 1000, 1e-4)
	fel.AddTiming("generate", "M1", 1000, 5e-5)
	fel.AddTiming("generate", "M2", 1000, 1e-5)

	// include the timings for the operations just specified 
	fel.AddTiming("rtt", "x86", 1000, 0.0)
	fel.AddTiming("rtt", "M1", 1000, 0.0)
	fel.AddTiming("rtt", "M2", 1000, 0.0)

	// include the timings for the operations just specified 
	fel.AddTiming("passThru", "x86", 1000, 1e-5)
	fel.AddTiming("passThru", "M1", 1000, 5e-6)
	fel.AddTiming("passThru", "M2", 1000, 1e-6)

	// include the timings for the operations just specified 
	fel.AddTiming("decrypt-aes", "x86", 1000, 1e-4)
	fel.AddTiming("decrypt-aes", "accel-x86", 1000, 1e-5)
	fel.AddTiming("decrypt-aes", "M1", 1000, 5e-5)
	fel.AddTiming("decrypt-aes", "M2", 1000, 1e-5)

	fel.AddTiming("decrypt-rsa", "x86", 1000, 1e-3)
	fel.AddTiming("decrypt-rsa", "M1", 1000, 5e-4)
	fel.AddTiming("decrypt-rsa", "M2", 1000, 1e-4)
	fel.AddTiming("decrypt-rsa", "accel-x86", 1000, 1e-6)
	// include the timings for the operations just specified 
	fel.AddTiming("encrypt-aes", "x86", 1000, 1e-3)
	fel.AddTiming("encrypt-aes", "accel-x86", 1000, 1e-3)
	fel.AddTiming("encrypt-aes", "M1", 1000, 5e-4)
	fel.AddTiming("encrypt-aes", "M2", 1000, 1e-5)
	fel.AddTiming("encrypt-rsa", "x86", 1000, 1e-2)
	fel.AddTiming("encrypt-rsa", "M1", 1000, 5e-3)
	fel.AddTiming("encrypt-rsa", "M2", 1000, 1e-3)


	// get the state initialization stuff
	cpfs, found := epCPInit.RecoverCPFuncState("encryptPerf")
	if found {
		cpFuncStateDict.AddCPFuncState(cpfs)
	}		
	cpFuncStateDict.WriteToFile(fullpathmap["cpState"])

	// write the comp pattern stuff out
	cpDict.WriteToFile(fullpathmap["cp"])
	cpInitDict.WriteToFile(fullpathmap["cpInit"])

    // build a topology named 'Protected' with two networks
    tcf := mrnes.CreateTopoCfgFrame("Protected")

	// the two networks
    pvtNet := mrnes.CreateNetworkFrame("private", "LAN", "wireless")
	pubNet := mrnes.CreateNetworkFrame("public", "LAN", "wired")

	pvtNet.AddGroup("LAN")
	pvtNet.AddGroup("wireless")
	pubNet.AddGroup("LAN")
	pubNet.AddGroup("wired")

	// create a source node in pubnet
	srcHost := mrnes.CreateHostFrame("src", "Dell")
	srcHost.AddGroup("source")
	pubNet.IncludeDev(srcHost, "wired", true)

	// create an SSL filter that joins pvtNet and pubNet
	sslFilter := mrnes.CreateFilterFrame("ssl", "Filter", "crypto")
	pubNet.IncludeDev(sslFilter, "wired", true)
	pvtNet.IncludeDev(sslFilter, "wired", true)

	// srcHost doesn't have a cable to sslFilter, but it does have a carry connection
	mrnes.ConnectDevs(srcHost, sslFilter, false, pubNet.Name)

	// create a router to sit between the hub and ssl
	pvtRtr := mrnes.CreateRouterFrame("pvtRtr", "cisco")

	// put the router in group "interior"
	pvtRtr.AddGroup("interior")
	pvtNet.IncludeDev(pvtRtr, "wired", true)

	// connect it to ssl with a cable
	mrnes.ConnectDevs(sslFilter, pvtRtr, true, pvtNet.Name)

	// create a wireless router to be a hub
	pvtHub := mrnes.CreateRouterFrame("pvtHub", "fortinet")

	// put the router in the group "hub"
	pvtHub.AddGroup("hub")

	// put the router in the group "access control"
	pvtHub.AddGroup("access control")	
	// connect the private router and hub
	mrnes.ConnectDevs(pvtRtr, pvtHub, true, pvtNet.Name)

	// create the hosts, add to the network, connect to hub
	for _, hostName := range allhosts {
		host := mrnes.CreateHostFrame(hostName, "Dell")
		pvtNet.IncludeDev(host,"wireless", true)
		pvtHub.WirelessConnectTo(host, pvtNet.Name)	
	}

	// put a CA in pvtNet connected directory to the pvtRtr
	CA := mrnes.CreateHostFrame("CA", "Dell")
	pvtNet.IncludeDev(CA, "wired", true)
	mrnes.ConnectDevs(pvtRtr, CA, true, pvtNet.Name)

	// include the networks in the topo configuration	
	tcf.AddNetwork(pubNet)
	tcf.AddNetwork(pvtNet)

    // fill in any missing parts needed for the topology description
    topoCfgerr := tcf.Consolidate()
    if topoCfgerr != nil {
        panic(topoCfgerr)
    }

    // turn the pointer-oriented data structures into a flat string-based
    // version for serialization, then save to file
    tc := tcf.Transform()

    tc.WriteToFile(fullpathmap["topo"])

	fel.WriteToFile(fullpathmap["funcExec"])

    // build a table of device timing parameters
    dev := mrnes.CreateDevExecList("BITS-1")
	dev.AddTiming("switch", "Slow", 1e-4)
	dev.AddTiming("switch", "Fast", 1e-5)
	dev.AddTiming("route", "Slow", 5e-3)
	dev.AddTiming("route", "Fast", 1e-3)

	dev.WriteToFile(fullpathmap["devExec"])
	
    // create the dictionary to be populated
    expCfg := mrnes.CreateExpCfg("BITS-1")
	mrnes.GetExpParamDesc()

    // experiment parameters are largely about architectural parameters
    // that impact performance. Define some defaults (which can be overwritten later)
    //

	// every wired interface to have a delay of 10e-6 
	as := mrnes.AttrbStruct{}
	as.AttrbName = "media"
	as.AttrbValue = "wired"
	attrbs := []mrnes.AttrbStruct{as} 
    expCfg.AddParameter("Interface", attrbs, "delay", "10e-6")

	// every wired network to have a latency of 10e-3
    expCfg.AddParameter("Network", attrbs, "latency", "10e-3")

	// every wired network to have a bandwidth of 1000 (Mbits)
    expCfg.AddParameter("Network", attrbs, "bandwidth", "1000")

	// every wireless interface to have a delay of 10e-4	
	as.AttrbName = "media"
	as.AttrbValue = "wireless"
	attrbs = []mrnes.AttrbStruct{as} 
    expCfg.AddParameter("Interface", attrbs, "latency", "10e-4")
    expCfg.AddParameter("Interface", attrbs, "delay", "10e-6")

	// every wireless network to have capacity of 1000Mbits
    expCfg.AddParameter("Network", attrbs, "capacity", "1000")

	// every wireless network to have bandwidth of 1000Mbits
    expCfg.AddParameter("Network", attrbs, "bandwidth", "1000")
	
	// every interface to have a bandwidth of 100 Mbits
	wcAttrbs := []mrnes.AttrbStruct{ mrnes.AttrbStruct{AttrbName:"*", AttrbValue:""}}
    expCfg.AddParameter("Interface", wcAttrbs, "bandwidth", "100")

	// every interface to have an MTU of 1500 bytes
    expCfg.AddParameter("Interface", wcAttrbs, "MTU", "1500")

	// every host to have an x86 cpu
    expCfg.AddParameter("Host", wcAttrbs, "CPU", "x86")

	// filter named "crypto" to have an "accel-x86" CPU
	as.AttrbName = "name"
	as.AttrbValue = "crypto"
	attrbs = []mrnes.AttrbStruct{as} 
    expCfg.AddParameter("Filter", attrbs, "CPU", "accel-x86")
    expCfg.AddParameter("Filter", attrbs, "accelerator", "true")

	// every host, switch, router to have trace set on
	expCfg.AddParameter("Host", wcAttrbs, "trace", "true")
	expCfg.AddParameter("Switch", wcAttrbs, "model", "Slow")
	expCfg.AddParameter("Switch", wcAttrbs, "trace", "true")
	expCfg.AddParameter("Router", wcAttrbs, "trace", "true")
	expCfg.AddParameter("Filter", wcAttrbs, "trace", "true")

	// every switch to have a buffer size of 100Mbytes
	expCfg.AddParameter("Switch", wcAttrbs, "buffer", "100")

	// every router to have a buffer size of 256Mbytes
	expCfg.AddParameter("Router", wcAttrbs, "buffer", "256")

	// every router to have model "Slow"
	expCfg.AddParameter("Router", wcAttrbs, "model", "Slow")
	expCfg.WriteToFile(fullpathmap["exp"])

	cmpMapDict := cmptn.CreateCompPatternMapDict("Maps")
	cmpMap := cmptn.CreateCompPatternMap("encryptPerf")
	cmpMap.AddMapping("src",	"src", false)
	cmpMap.AddMapping("crypto", "ssl", false)
    for _, hostName := range allhosts {
		cmpMap.AddMapping(hostName, hostName, false)
	}
	cmpMapDict.AddCompPatternMap(cmpMap, false)	
	cmpMapDict.WriteToFile(fullpathmap["map"])	
}

