package main

// code to build template of MrNesbits CompPattern structures and their initialization structs

import (
	"fmt"
	"github.com/iti/cmdline"
	cmptn "github.com/iti/mrnesbits"
	"path"
	"path/filepath"
)

// cmdlineParams defines the parameters recognized
// on the command line
func cmdlineParams() *cmdline.CmdParser {
	// command line parameters are all about file and directory locations.
	// Even though we don't need the flags for the other MrNesbits structures we
	// keep them here so that all the programs that build templates can use the same arguments file
	// create an argument parser
	cp := cmdline.NewCmdParser()
	cp.AddFlag(cmdline.StringFlag, "prbLib", true)       // directory of tmplates
	cp.AddFlag(cmdline.StringFlag, "cpPrb", true)        // name of output file used for comp pattern templates
	cp.AddFlag(cmdline.StringFlag, "expPrb", false)      // name of output file used for expCfg templates
	cp.AddFlag(cmdline.StringFlag, "funcExecPrb", false) // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "devExecPrb", false)  // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "prbCSV", false)      // name of output file used for exec templates
	cp.AddFlag(cmdline.StringFlag, "topoPrb", false)     // name of output file used for topo templates
	cp.AddFlag(cmdline.StringFlag, "cpInitPrb", true)    // name of output file used for func parameters templates

	return cp
}

// main gives the entry point
func main() {
	// define the command line parameters
	cp := cmdlineParams()

	// parse the command line
	cp.Parse()

	// string for the template library
	prbLibDir := cp.GetVar("prbLib").(string)

	// make sure this directory exists
	dirs := []string{prbLibDir}
	valid, err := cmptn.CheckDirectories(dirs)
	if !valid {
		panic(err)
	}

	// check for access to template output files
	cpPrbFile := cp.GetVar("cpPrb").(string)
	CPInitPrbFile := cp.GetVar("cpInitPrb").(string)

	// join directory specifications with file name specifications
	cpPrbFile = filepath.Join(prbLibDir, cpPrbFile) // path to file holding template files of comp patterns

	CPInitPrbFile = filepath.Join(prbLibDir, CPInitPrbFile) // path to file holding template files of comp patterns

	// notice whether the initialization struct should be serialized as yaml
	CPInitExt := path.Ext(CPInitPrbFile)
	CPInitOutputIsYAML := (CPInitExt == ".yaml" || CPInitExt == ".YAML" || CPInitExt == ".yml")

	// check the validity of files to be created
	files := []string{cpPrbFile, CPInitPrbFile}
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
	cpPrbDict := cmptn.CreateCompPatternDict("CompPatterns-1", true)
	cpInitPrbDict := cmptn.CreateCPInitListDict("CompPatterns-1", true)

	// create a CompPattern with stateful functions
	StatefulFlag := cmptn.CreateCompPattern("StatefulTest")

	// 'src' will periodically generate a packet that carries a 'valid'
	// bit, and every so often flip the bit to 'invalid'
	src := cmptn.CreateFunc("mark", "src", "stateful")

	// the packet will be directed to the 'branch' function, which
	// will be configured to test the bit or not.
	branch := cmptn.CreateFunc("testFlag", "branch", "stateful")

	// the packet is routed to consumer1 if either 'test' does not
	// test the bit or if it does and the bit is valid
	consumer1 := cmptn.CreateFunc("process", "consumer1", "static")

	// the packet will be routed to consumer2 if 'test' examines the
	// valid bit and sees that it is false
	consumer2 := cmptn.CreateFunc("process", "consumer2", "static")

	// include these functions in the CompPattern
	StatefulFlag.AddFunc(src)
	StatefulFlag.AddFunc(branch)
	StatefulFlag.AddFunc(consumer1)
	StatefulFlag.AddFunc(consumer2)

	// self-initiation message has type 'initiate', and also label 'initiate'
	StatefulFlag.AddEdge(src.Label, src.Label, "initiate", "initiate")

	// packet with valid bit goes from to src to branch with message of type 'marked'
	StatefulFlag.AddEdge(src.Label, branch.Label, "marked", "marked")

	// possible to go from branch func to consumer1 func, message type is 'result', label is 'consumer1'
	StatefulFlag.AddEdge(branch.Label, consumer1.Label, "result", "consumer1")

	// possible to go from branch func to consumer2 func, message type is also 'result', label is 'consumer2'
	StatefulFlag.AddEdge(branch.Label, consumer2.Label, "result", "consumer2")

	// done with the first CompPattern topology, add to the dictionary of CompPatterns
	cpPrbDict.AddCompPattern(StatefulFlag, true, false)

	// now create initialization structs for each of the StatefulFlag CP functions.
	// Start making a data structure for them, remember whether to push the initation structs out in
	// YAML form
	DFCPInit := cmptn.CreateCPInitList("StatefulTest", "StatefulTest", CPInitOutputIsYAML)

	// flesh out the messages. Each type has a packetlength of 250 bytes and a message length of
	// 1500 bytes
	DFInitMsg := cmptn.CreateCompPatternMsg("initiate", 250, 1500)
	DFMarkedMsg := cmptn.CreateCompPatternMsg("marked", 250, 1500)
	DFResultMsg := cmptn.CreateCompPatternMsg("result", 250, 1500)

	// remember these message specifications
	DFCPInit.AddMsg(DFInitMsg)
	DFCPInit.AddMsg(DFMarkedMsg)
	DFCPInit.AddMsg(DFResultMsg)

	srcActions := []cmptn.ActionResp{}
	srcInEdge := cmptn.InEdge{SrcLabel:src.Label, MsgType: DFInitMsg.MsgType}
	srcActionDesc := cmptn.ActionDesc{Select: "mark", CostType: "mark"}
	srcActionResp := cmptn.CreateActionResp(srcInEdge, srcActionDesc, 0)
	srcActions = append(srcActions, srcActionResp)

	// make a state block for the 'src' function
	srcState := make(map[string]string)
	srcState["failperiod"] = "1000"
	srcParams := cmptn.CreateStatefulParameters(StatefulFlag.CPType, src.Label,  srcActions, srcState)

	// src has period 2.0, meaning self initiation every 2 seconds. The response is
	// mapping input an input from self with message type "initiate" to func 'branch' with message having type "marked"
	srcOutEdge := cmptn.OutEdge{DstLabel: branch.Label, MsgType: DFMarkedMsg.MsgType}
	srcParams.AddResponse(srcInEdge, srcOutEdge, srcOutEdge.MsgType, 2.0)

	// serialize and save initialization struct for src
	srcParamStr, _ := srcParams.Serialize(CPInitOutputIsYAML)
	DFCPInit.AddParam(src.Label, src.ExecType, srcParamStr)

	// create state for 'branch' func
	branchState := make(map[string]string)
	branchState["test"] = "true" // select testing certificate flag
	branchActions := []cmptn.ActionResp{}
	branchInEdge := cmptn.InEdge{SrcLabel:src.Label, MsgType: DFMarkedMsg.MsgType}
	branchActionDesc := cmptn.ActionDesc{Select: "testFlag", CostType: "testFlag"}
	branchActionResp := cmptn.CreateActionResp(branchInEdge, branchActionDesc, 0)
	branchActions = append(branchActions, branchActionResp)

	branchParams := cmptn.CreateStatefulParameters(StatefulFlag.CPType, branch.Label,  branchActions, branchState)
	branchOutEdge1 := cmptn.OutEdge{DstLabel: consumer1.Label, MsgType: DFResultMsg.MsgType}
	branchParams.AddResponse(branchInEdge, branchOutEdge1, branchOutEdge1.MsgType, 0)
	branchOutEdge2 := cmptn.OutEdge{DstLabel: consumer2.Label, MsgType: DFResultMsg.MsgType}
	branchParams.AddResponse(branchInEdge, branchOutEdge2, branchOutEdge2.MsgType, 0)
	branchParamStr, _ := branchParams.Serialize(CPInitOutputIsYAML)
	DFCPInit.AddParam(branch.Label, branch.ExecType, branchParamStr)

	// initialization parameters for customer1 and customer2 are to just consume the message and not generate a response
	consumer1Params := cmptn.CreateStaticParameters(StatefulFlag.CPType, consumer1.Label)
	consumer1InEdge := cmptn.InEdge{SrcLabel: branch.Label, MsgType: DFResultMsg.MsgType}
	consumerOutEdge := cmptn.OutEdge{DstLabel:"", MsgType: ""}
	consumer1Params.AddResponse(consumer1InEdge, consumerOutEdge, consumerOutEdge.MsgType, 0)
	consumer1ParamStr, _ := consumer1Params.Serialize(CPInitOutputIsYAML)
	DFCPInit.AddParam(consumer1.Label, consumer1.ExecType, consumer1ParamStr)

	consumer2Params := cmptn.CreateStaticParameters(StatefulFlag.CPType, consumer2.Label)
	consumer2InEdge := cmptn.InEdge{SrcLabel: branch.Label, MsgType: DFResultMsg.MsgType}
	consumer2Params.AddResponse(consumer2InEdge, consumerOutEdge, consumerOutEdge.MsgType, 0)
	consumer2ParamStr, _ := consumer2Params.Serialize(CPInitOutputIsYAML)
	DFCPInit.AddParam(consumer2.Label, consumer2.ExecType, consumer2ParamStr)
	DFCPInit.AddParam(consumer2.Label, consumer2.ExecType, consumer2ParamStr)

	// the initialization for the "StatefulTest" is done, save it in the initialization dictionary
	cpInitPrbDict.AddCPInitList(DFCPInit, false)

	// the random branch example has two self-initiating sources that both feed
	// a random branch func.  That func has two consumers, and randomly chooses
	// between them in response to an input.  The probability distribution used
	// depends on on which generator created the message

	// The RSA (and AES) chain CompPatterns are identical in struction.
	// - a self-initiating func  generates a 'data' message and sends to
	// - a function which models the resource consumption of encrypting that message, sending the result to
	// - a function which models the resource consumption of decrtypting that message, sending the result to
	// - a function that consumes the result

	RSAChain := cmptn.CreateCompPattern("RSAChain")

	// the four nodes. Note that function type (e.g., "RSA-encrypt") denote general type,
	// while label is an identifier for this CompPattern
	src = cmptn.CreateFunc("generate", "src", "static")
	encrypt := cmptn.CreateFunc("RSA-encrypt", "encrypt", "static")
	decrypt := cmptn.CreateFunc("RSA-decrypt", "decrypt", "static")
	sink := cmptn.CreateFunc("consume", "sink", "static")

	// add to the CompPattern
	RSAChain.AddFunc(src)
	RSAChain.AddFunc(encrypt)
	RSAChain.AddFunc(decrypt)
	RSAChain.AddFunc(sink)

	// include edges.
	RSAChain.AddEdge(src.Label, src.Label, "initiate", "initiate")
	RSAChain.AddEdge(src.Label, encrypt.Label, "data", "data")
	RSAChain.AddEdge(encrypt.Label, decrypt.Label, "encrypted", "encrypted")
	RSAChain.AddEdge(decrypt.Label, sink.Label, "decrypted", "decrypted")

	// add the RSAChain to the CompPatternDict
	cpPrbDict.AddCompPattern(RSAChain, true, false)
	fmt.Println("Added RSAChain model to pattern dictionary")

	// Add initialization parameters to RSAChain CompPattern
	RSACPInit := cmptn.CreateCPInitList("RSAChain", "RSAChain", CPInitOutputIsYAML)

	RSAInitMsg := cmptn.CreateCompPatternMsg("initiate", 50, 1500)      // self-initiation
	RSADataMsg := cmptn.CreateCompPatternMsg("data", 1000, 1500)        // generated pckt
	RSAEncryptMsg := cmptn.CreateCompPatternMsg("encrypted", 500, 1500) // encrypted pckt
	RSADecryptMsg := cmptn.CreateCompPatternMsg("decrypted", 500, 1500) // decrypted pckt for consumption

	RSACPInit.AddMsg(RSAInitMsg)
	RSACPInit.AddMsg(RSADataMsg)
	RSACPInit.AddMsg(RSAEncryptMsg)
	RSACPInit.AddMsg(RSADecryptMsg)

	// Now add function responses.
	// create parameter block variable for src
	srcRSAParams := cmptn.CreateStaticParameters(RSAChain.CPType, src.Label)

	// self-initiate, message src -> encrypt every 1.0 second
	srcInEdge  = cmptn.InEdge{SrcLabel: src.Label, MsgType: RSAInitMsg.MsgType}
	srcOutEdge = cmptn.OutEdge{DstLabel: encrypt.Label, MsgType: RSADataMsg.MsgType}
	srcRSAParams.AddResponse(srcInEdge, srcOutEdge, srcOutEdge.MsgType, 1.0)
	srcParamStr, _ = srcRSAParams.Serialize(CPInitOutputIsYAML)
	RSACPInit.AddParam(src.Label, src.ExecType, srcParamStr)

	// create parameter block variable for encrypt
	encryptParams := cmptn.CreateStaticParameters(RSAChain.CPType, encrypt.Label)
	encryptInEdge  := cmptn.InEdge{SrcLabel: src.Label, MsgType: RSADataMsg.MsgType}
	encryptOutEdge := cmptn.OutEdge{DstLabel: decrypt.Label, MsgType: RSAEncryptMsg.MsgType}
	encryptParams.AddResponse(encryptInEdge, encryptOutEdge, encryptOutEdge.MsgType, 0.0)
	encryptParamStr, _ := encryptParams.Serialize(CPInitOutputIsYAML)
	RSACPInit.AddParam(encrypt.Label, encrypt.ExecType, encryptParamStr)

	// create parameter block variable for decrypt
	decryptParams := cmptn.CreateStaticParameters(RSAChain.CPType, decrypt.Label)
	decryptInEdge  := cmptn.InEdge{SrcLabel: encrypt.Label, MsgType: RSAEncryptMsg.MsgType}
	decryptOutEdge := cmptn.OutEdge{DstLabel: sink.Label, MsgType: RSADecryptMsg.MsgType}
	decryptParams.AddResponse(decryptInEdge, decryptOutEdge, decryptOutEdge.MsgType, 0.0)
	decryptParamStr, _ := decryptParams.Serialize(CPInitOutputIsYAML)
	RSACPInit.AddParam(decrypt.Label, decrypt.ExecType, decryptParamStr)

	// create parameter block variable for sink
	sinkParams := cmptn.CreateStaticParameters(RSAChain.CPType, sink.Label)
	sinkInEdge  := cmptn.InEdge{SrcLabel: decrypt.Label, MsgType: RSADecryptMsg.MsgType}
	sinkOutEdge := cmptn.OutEdge{DstLabel: "", MsgType: ""}
	sinkParams.AddResponse(sinkInEdge, sinkOutEdge, sinkOutEdge.MsgType, 0.0)
	sinkParamStr, _ := sinkParams.Serialize(CPInitOutputIsYAML)
	RSACPInit.AddParam(sink.Label, sink.ExecType, sinkParamStr)

	// add the RSAChain Initialization to the dictionary
	cpInitPrbDict.AddCPInitList(RSACPInit, false)

	// Add a client-server CompPattern w/o certificates
	// Just two functions, the client (self-initiating) and server
	SimpleWeb := cmptn.CreateCompPattern("SimpleWeb")

	// two funcs
	webClient := cmptn.CreateFunc("generate", "webClient", "static")
	webServer := cmptn.CreateFunc("serve", "webServer", "static")
	SimpleWeb.AddFunc(webClient)
	SimpleWeb.AddFunc(webServer)

	// three edges: one to self-initiate, one to request service, the other to report service provided
	SimpleWeb.AddEdge(webClient.Label, webClient.Label, "initiate", "initiate")
	SimpleWeb.AddEdge(webClient.Label, webServer.Label, "request", "request")
	SimpleWeb.AddEdge(webServer.Label, webClient.Label, "response", "response")

	// initialization structure for SimpleWeb
	SimpleWebCPInit := cmptn.CreateCPInitList("SimpleWeb", "SimpleWeb", CPInitOutputIsYAML)

	// three message types
	SimpleWebInitMsg := cmptn.CreateCompPatternMsg("initiate", 100, 1500)
	SimpleWebReqMsg := cmptn.CreateCompPatternMsg("request", 250, 1500)
	SimpleWebRspMsg := cmptn.CreateCompPatternMsg("response", 1500, 1500)
	SimpleWebCPInit.AddMsg(SimpleWebInitMsg)
	SimpleWebCPInit.AddMsg(SimpleWebReqMsg)
	SimpleWebCPInit.AddMsg(SimpleWebRspMsg)

	// when "webClient" gets message of type "initiate" from self, respond by
	// sending "request" message to "webServer".  Every 3.333 seconds...
	webParams := cmptn.CreateStaticParameters(SimpleWeb.CPType, webClient.Label)
	webClientInEdge := cmptn.InEdge{SrcLabel: webClient.Label, MsgType: SimpleWebInitMsg.MsgType}
	webClientOutEdge := cmptn.OutEdge{DstLabel: webServer.Label, MsgType: SimpleWebReqMsg.MsgType}
	webParams.AddResponse(webClientInEdge, webClientOutEdge, webClientOutEdge.MsgType, 3.333)
	webClientInEdge = cmptn.InEdge{SrcLabel: webServer.Label, MsgType: SimpleWebRspMsg.MsgType}
	emptyEdge := cmptn.OutEdge{DstLabel: "", MsgType: ""}
	webParams.AddResponse(webClientInEdge, emptyEdge, "", 0.0)

	// save the webServer initialization structure
	webParamStr, _ := webParams.Serialize(CPInitOutputIsYAML)
	SimpleWebCPInit.AddParam(webClient.Label, webClient.ExecType, webParamStr)

	// when "webServer" gets message of type "request" from "webClient", respond by
	// sending "response" message to "webClient"
	webServerInEdge := cmptn.InEdge{SrcLabel: webClient.Label, MsgType: SimpleWebReqMsg.MsgType}
	webServerOutEdge := cmptn.OutEdge{DstLabel: webClient.Label, MsgType: SimpleWebRspMsg.MsgType}

	webParams = cmptn.CreateStaticParameters(SimpleWeb.CPType, webServer.Label)
	webParams.AddResponse(webServerInEdge, webServerOutEdge, webServerOutEdge.MsgType, 0)

	// just one response to save, so serialize the func's initiationization struct
	webParamStr, _ = webParams.Serialize(CPInitOutputIsYAML)
	SimpleWebCPInit.AddParam(webServer.Label, webServer.ExecType, webParamStr)

	// all the functions have their initialization structs built and committed, put
	// the SimpleWeb wrapper into the dictionary
	cpInitPrbDict.AddCPInitList(SimpleWebCPInit, false)

	// add the CompPattern topology to the dictionary
	cpPrbDict.AddCompPattern(SimpleWeb, true, false)
	fmt.Println("Added client-server model to pattern dictionary")

	// Add a client-server pattern with simple certificate serving.
	// Four functions: client, server, pki-cache, pki-server
	PKIWeb := cmptn.CreateCompPattern("PKIWeb")
	pkiClient := cmptn.CreateFunc("generate", "web-pkiClient", "static")
	pkiServer := cmptn.CreateFunc("pki-pkiServer", "web-pkiServer", "stateful")
	pkiCache := cmptn.CreateFunc("pki-pkiCache", "pkiCache", "stateful")
	certSrvr := cmptn.CreateFunc("cert-pkiServer", "certs", "static")

	PKIWeb.AddFunc(pkiClient)
	PKIWeb.AddFunc(pkiServer)
	PKIWeb.AddFunc(pkiCache)
	PKIWeb.AddFunc(certSrvr)

	PKIWeb.AddEdge(pkiClient.Label, pkiClient.Label, "initiate", "initiate")
	PKIWeb.AddEdge(pkiClient.Label, pkiServer.Label, "request", "web-request")
	PKIWeb.AddEdge(pkiServer.Label, pkiCache.Label, "request", "cert-request")
	PKIWeb.AddEdge(pkiCache.Label, certSrvr.Label, "request", "cert-request")
	PKIWeb.AddEdge(certSrvr.Label, pkiCache.Label, "response", "cert-response")
	PKIWeb.AddEdge(pkiCache.Label, pkiServer.Label, "response", "cert-response")
	PKIWeb.AddEdge(pkiServer.Label, pkiClient.Label, "response", "web-response")

	// add the PKIWeb to the CompPatternDict
	cpPrbDict.AddCompPattern(PKIWeb, true, false)
	fmt.Println("Added PKIWeb with certs model to pattern dictionary")

	// three message types
	PKIInitMsg := cmptn.CreateCompPatternMsg("initiate", 100, 1500)
	PKIReqMsg := cmptn.CreateCompPatternMsg("request", 250, 1500)
	PKIRspMsg := cmptn.CreateCompPatternMsg("response", 1500, 1500)

	// create the CompPattern's initialization structure and stick in the messages
	PKIWebCPInit := cmptn.CreateCPInitList("PKIWeb", "PKIWeb", CPInitOutputIsYAML)
	PKIWebCPInit.AddMsg(PKIInitMsg)
	PKIWebCPInit.AddMsg(PKIReqMsg)
	PKIWebCPInit.AddMsg(PKIRspMsg)

	// build the InEdge and OutEdge references used in building up the action and response tables
	pkiClientFromClientEdge := cmptn.InEdge{SrcLabel: pkiClient.Label, MsgType: PKIInitMsg.MsgType}
	pkiClientFromServerEdge := cmptn.InEdge{SrcLabel: pkiServer.Label, MsgType: PKIRspMsg.MsgType}

	pkiServerFromClientEdge := cmptn.InEdge{SrcLabel: pkiClient.Label, MsgType: PKIReqMsg.MsgType}
	pkiServerFromCacheEdge  := cmptn.InEdge{SrcLabel: pkiCache.Label, MsgType: PKIRspMsg.MsgType}

	pkiCacheFromServerEdge  := cmptn.InEdge{SrcLabel: pkiServer.Label, MsgType: PKIReqMsg.MsgType}
	pkiCacheFromCertSrvrEdge  := cmptn.InEdge{SrcLabel: certSrvr.Label, MsgType: PKIRspMsg.MsgType}

	pkiCertSrvrFromCacheEdge := cmptn.InEdge{SrcLabel: pkiCache.Label, MsgType: PKIRspMsg.MsgType}

	pkiClientToServerEdge := cmptn.OutEdge{DstLabel: pkiServer.Label, MsgType: PKIReqMsg.MsgType}
	pkiServerToClientEdge := cmptn.OutEdge{DstLabel: pkiClient.Label, MsgType: PKIRspMsg.MsgType}
	pkiServerToCacheEdge := cmptn.OutEdge{DstLabel: pkiCache.Label, MsgType: PKIReqMsg.MsgType}

	pkiCacheToServerEdge   := cmptn.OutEdge{DstLabel: pkiServer.Label, MsgType: PKIRspMsg.MsgType}
	pkiCacheToCertSrvrEdge := cmptn.OutEdge{DstLabel: certSrvr.Label, MsgType: PKIReqMsg.MsgType}

	pkiCertSrvrToCacheEdge := cmptn.OutEdge{DstLabel: pkiCache.Label, MsgType: PKIRspMsg.MsgType}


	// build the response function descriptions (code for function to call on prompt, code for execution cost)
	// These are needed only for stateful functions, i.e., pkiServer and pkiCache. The ActionResps resulting
	// are included when we build the function's parameter initialization structs

	// pkiServer's functional respose to a message from pkiClient
	pkiServerFromClientActionDesc := cmptn.ActionDesc{Select: "filter", CostType: "filter"}

	// ActionResp connects InEdge with a functional response.  No "choice" label, no limit 
	pkiServerFromClientActionResp := cmptn.CreateActionResp(pkiServerFromClientEdge, pkiServerFromClientActionDesc, 0) 
	
	// pkiServer's functional respose to a message from pkiCache
	pkiServerFromCacheActionDesc := cmptn.ActionDesc{Select: "passThru", CostType: "passThru"}

	// ActionResp connects InEdge with a functional response.  No "choice" label, no limit 
	pkiServerFromCacheActionResp := cmptn.CreateActionResp(pkiServerFromCacheEdge, pkiServerFromCacheActionDesc, 0) 
		
	// pkiCache's functional respose to a message from pkiClient
	pkiCacheFromServerActionDesc := cmptn.ActionDesc{Select: "filter", CostType: "filter"}

	// ActionResp connects InEdge with a functional response.  No "choice" label, no limit 
	pkiCacheFromServerActionResp := cmptn.CreateActionResp(pkiCacheFromServerEdge, pkiCacheFromServerActionDesc, 0) 
	
	// pkiCache's functional respose to a message from pkiCertSrvr
	pkiCacheFromCertSrvrActionDesc := cmptn.ActionDesc{Select: "passThru", CostType: "passThru"}

	// ActionResp connects InEdge with a functional response.  No "choice" label, no limit 
	pkiCacheFromCertSrvrActionResp := cmptn.CreateActionResp(pkiCacheFromCertSrvrEdge, pkiCacheFromCertSrvrActionDesc, 0) 
		
	// initialize the pkiClient.
	pkiClientParams := cmptn.CreateStaticParameters(PKIWeb.CPType, pkiClient.Label)

	// pkiClient response to initiate message, self-initiate every 1.5 seconds
	pkiClientParams.AddResponse(pkiClientFromClientEdge, pkiClientToServerEdge, pkiClientToServerEdge.MsgType, 1.5)

	// pkiClient response to returned response from pkiServer
	pkiClientParams.AddResponse(pkiClientFromServerEdge, emptyEdge, "", 0.0)
	pkiClientParamStr, _ := pkiClientParams.Serialize(CPInitOutputIsYAML)

	// save the pkiClients's initialization
	PKIWebCPInit.AddParam(pkiClient.Label, pkiClient.ExecType, pkiClientParamStr)

	// create the initialization block for the pkiServer
	// state block holds threshold "cach-interval" dictating how often to forward requests
	// to pkiCache, and "visits" which is the number times requests have been seen coming from the pkiClient
	pkiServerState := make(map[string]string)
	pkiServerState["threshold"] = "1000" // forward every 1000th request
	pkiServerState["visits"] = "0"       // number of visits to this function

	// bring in the action from the Client
	pkiServerActions := []cmptn.ActionResp{pkiServerFromClientActionResp}

	// bring in the action from the Cache
	pkiServerActions = append(pkiServerActions, pkiServerFromCacheActionResp)

	// create the initialization block
	statefulparams := cmptn.CreateStatefulParameters(PKIWeb.CPType, pkiServer.Label, pkiServerActions, pkiServerState)

	// one response is directly back to the pkiClient
	statefulparams.AddResponse(pkiServerFromClientEdge, pkiServerToClientEdge, pkiServerToClientEdge.MsgType, 0.0)
	
	// one response forwards a request to the pkiCache.  This response is marked to distinguish from the other in the response selection logic
	statefulparams.AddResponse(pkiServerFromClientEdge, pkiServerToCacheEdge, pkiServerToCacheEdge.MsgType, 0.0)

	// bundle up the initialization structure and save it for this CompPattern
	statefulparamStr, _ := statefulparams.Serialize(CPInitOutputIsYAML)
	PKIWebCPInit.AddParam(pkiServer.Label, pkiServer.ExecType, statefulparamStr)

	// create the initialization block for the pkiCache
	pkiCacheState := make(map[string]string)
	pkiCacheState["threshold"] = "100"  // forward every 100th request
	pkiCacheState["visits"] = "0"       // number of visits to this function

	// bring in the action from the Cache
	pkiCacheActions := []cmptn.ActionResp{pkiCacheFromServerActionResp}

	// bring in the action from the certSrvr
	pkiCacheActions = append(pkiCacheActions, pkiCacheFromCertSrvrActionResp)

	// create the initialization block
	statefulparams = cmptn.CreateStatefulParameters(PKIWeb.CPType, pkiCache.Label, pkiCacheActions, pkiCacheState)

	// one response is directly back to the pkiClient
	statefulparams.AddResponse(pkiCacheFromServerEdge, pkiCacheToServerEdge, pkiCacheToServerEdge.MsgType, 0.0)
	
	// one response forwards a request to the pkiCertSrvr.  This response is marked to distinguish from the other in the response selection logic
	statefulparams.AddResponse(pkiCacheFromServerEdge, pkiCacheToCertSrvrEdge, pkiCacheToCertSrvrEdge.MsgType, 0.0)

	// bundle up the initialization structure and save it for this CompPattern
	statefulparamStr, _ = statefulparams.Serialize(CPInitOutputIsYAML)
	PKIWebCPInit.AddParam(pkiCache.Label, pkiCache.ExecType, statefulparamStr)

	// initialization for the certSrvr
	certSrvrParams := cmptn.CreateStaticParameters(PKIWeb.CPType, certSrvr.Label)

	// just one response possible, return result
	certSrvrParams.AddResponse(pkiCertSrvrFromCacheEdge, pkiCertSrvrToCacheEdge, pkiCertSrvrToCacheEdge.MsgType, 0.0)

	// wrap up the certSrvr responses
	certSrvrParamStr, _ := certSrvrParams.Serialize(CPInitOutputIsYAML)
	PKIWebCPInit.AddParam(certSrvr.Label, certSrvr.ExecType, certSrvrParamStr)

	// put the PKIWeb initialization structure into the initialization dictionary
	cpInitPrbDict.AddCPInitList(PKIWebCPInit, false)

	// write the computation patterns out to file
	cpPrbDict.WriteToFile(cpPrbFile)

	// write the initialization out to file
	cpInitPrbDict.WriteToFile(CPInitPrbFile)

	fmt.Printf("Output files %s, %s written!\n", cpPrbFile, CPInitPrbFile)
}

