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
	CPInitPrbFile := cp.GetVar("CPInitPrb").(string)

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
	cpPrbDict := cmptn.CreateCompPatternDict("CompPatterns-11", true)
	CPInitPrbDict := cmptn.CreateCPInitListDict("CompPatterns-1", true)

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

	// make a state block for the 'src' function
	dfstate := make(map[string]string)

	// one state variable, "failperiod", set to mark the bit as invalid on every 1000th packet
	dfstate["failperiod"] = "1000"

	// create a container for 'src's parameters.  Parameters are
	//	- the CompPattern type
	//  - the label of the function whose initialization struct we're building
	//  - a string code that identifies what code to execute
	//  - a string-to-string map that serves as state memory
	dfparams := cmptn.CreateStatefulParameters(StatefulFlag.CPType, src.Label, "mark", dfstate)

	// src has period 2.0, meaning self initiation every 2 seconds. The response is
	// mapping input an input from self with message type "initiate" to func 'branch' with message having type "marked"
	dfparams.AddResponse(src.Label, DFInitMsg.MsgType, DFMarkedMsg.MsgType, branch.Label, 2.0)

	// serialize and save initialization struct for src
	dfparamStr, _ := dfparams.Serialize(CPInitOutputIsYAML)
	DFCPInit.AddParam(src.Label, dfparamStr)

	// create state for 'branch' func
	dfstate = make(map[string]string)
	dfstate["testflag"] = "true" // select testing certificate flag

	// initialization parameters select mapping to customer1 or customer2
	dfparams = cmptn.CreateStatefulParameters(StatefulFlag.CPType, branch.Label, "testFlag", dfstate)

	dfparams.AddResponse(src.Label, DFMarkedMsg.MsgType, DFResultMsg.MsgType, consumer1.Label, 0.0)
	dfparams.AddResponse(src.Label, DFMarkedMsg.MsgType, DFResultMsg.MsgType, consumer2.Label, 0.0)
	dfparamStr, _ = dfparams.Serialize(CPInitOutputIsYAML)
	DFCPInit.AddParam(branch.Label, dfparamStr)

	// initialization parameters for customer1 and customer2 are to just consume the message and not generate a response
	sparams := cmptn.CreateStaticParameters(StatefulFlag.CPType, consumer1.Label)
	sparams.AddResponse(consumer1.Label, DFResultMsg.MsgType, "", "", 0.0)
	sparamStr, _ := sparams.Serialize(CPInitOutputIsYAML)
	DFCPInit.AddParam(consumer1.Label, sparamStr)

	sparams = cmptn.CreateStaticParameters(StatefulFlag.CPType, consumer2.Label)
	sparams.AddResponse(consumer2.Label, DFResultMsg.MsgType, "", "", 0.0)
	sparamStr, _ = sparams.Serialize(CPInitOutputIsYAML)
	DFCPInit.AddParam(consumer2.Label, sparamStr)

	// the initialization for the "StatefulTest" is done, save it in the initialization dictionary
	CPInitPrbDict.AddCPInitList(DFCPInit, false)

	// the random branch example has two self-initiating sources that both feed
	// a random branch func.  That func has two consumers, and randomly chooses
	// between them in response to an input.  The probability distribution used
	// depends on on which generator created the message

	// new CompPattern, demonstrating random function type
	RndBranch := cmptn.CreateCompPattern("RandomBranch")

	// four functions
	src1 := cmptn.CreateFunc("generate", "src1", "static")
	src2 := cmptn.CreateFunc("generate", "src2", "static")

	// notice that branch is 'random'
	branch = cmptn.CreateFunc("branch", "select", "random")

	consumer1 = cmptn.CreateFunc("consume", "consumer1", "static")
	consumer2 = cmptn.CreateFunc("consume", "consumer2", "static")
	RndBranch.AddFunc(src1)
	RndBranch.AddFunc(src2)
	RndBranch.AddFunc(branch)
	RndBranch.AddFunc(consumer1)
	RndBranch.AddFunc(consumer2)

	// src1 and src2 are self-initiating
	RndBranch.AddEdge(src1.Label, src1.Label, "initiate", "initiate")
	RndBranch.AddEdge(src2.Label, src2.Label, "initiate", "initiate")

	// src1 and src2 both direct their response to 'branch' func
	RndBranch.AddEdge(src1.Label, branch.Label, "data", "data")
	RndBranch.AddEdge(src2.Label, branch.Label, "data", "data")

	// 'branch' directs its response to either consumer1 or consumer2
	RndBranch.AddEdge(branch.Label, consumer1.Label, "selected", "consumer1")
	RndBranch.AddEdge(branch.Label, consumer2.Label, "selected", "consumer2")
	cpPrbDict.AddCompPattern(RndBranch, true, false)

	// create an initialization block for RandomBranch
	RBCPInit := cmptn.CreateCPInitList("RandomBranch", "RandomBranch", CPInitOutputIsYAML)

	// self-initiating message, data message from generators to branch, consume messages from branch to consumers
	RBInitMsg := cmptn.CreateCompPatternMsg("initiate", 50, 1500)
	RBDataMsg := cmptn.CreateCompPatternMsg("data", 1000, 1500)
	RBConsumeMsg := cmptn.CreateCompPatternMsg("consume", 1000, 1500)
	RBCPInit.AddMsg(RBInitMsg)
	RBCPInit.AddMsg(RBDataMsg)
	RBCPInit.AddMsg(RBConsumeMsg)

	// now create list of responses
	params := cmptn.CreateStaticParameters(RndBranch.CPType, src1.Label)

	// when src1 gets a message of type "initiate" from self, respond by
	// generating a message of type "data" and send to function branch. Time between
	// successive self-generations is 1.1
	params.AddResponse(src1.Label, RBInitMsg.MsgType, RBDataMsg.MsgType, branch.Label, 1.1)

	// serialize this initialization struct for src1 and save it
	paramStr, _ := params.Serialize(CPInitOutputIsYAML)
	RBCPInit.AddParam(src1.Label, paramStr)

	// when src2 gets a message of type "initiate" from self, respond by
	// generating a message of type "data" and send to function branch. Time between
	// successive self-generations is 2.125
	params = cmptn.CreateStaticParameters(RndBranch.CPType, src2.Label)
	params.AddResponse(src2.Label, RBInitMsg.MsgType, RBDataMsg.MsgType, branch.Label, 2.125)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	RBCPInit.AddParam(src2.Label, paramStr)

	// now the initialization parameters for 'branch'
	rndparams := cmptn.CreateRndParameters(RndBranch.CPType, branch.Label)

	// the probability distribution for branch output assigns probability mass
	// to an OutEdgeStruct, so create two options for OutEdgeStruct
	edgeToConsumer1_1 := cmptn.OutEdgeStruct{MsgType: RBConsumeMsg.MsgType, DstLabel: consumer1.Label}
	edgeToConsumer1_2 := cmptn.OutEdgeStruct{MsgType: RBConsumeMsg.MsgType, DstLabel: consumer2.Label}

	// assign probability masses to them
	ms1 := make(map[cmptn.OutEdgeStruct]float64)
	ms1[edgeToConsumer1_1] = 0.25
	ms1[edgeToConsumer1_2] = 0.75

	// notice that for a random func the 'AddResponse' arguments include probability mass function
	rndparams.AddResponse(src1.Label, RBDataMsg.MsgType, ms1)

	// a different response is included when the input edge is from src2, which means a different probability distribution can be used

	// here we're naming the output edges differently even though in this case they happen to be the same,
	// just to show that they can be different for a different response
	edgeToConsumer2_1 := cmptn.OutEdgeStruct{MsgType: RBConsumeMsg.MsgType, DstLabel: consumer1.Label}
	edgeToConsumer2_2 := cmptn.OutEdgeStruct{MsgType: RBConsumeMsg.MsgType, DstLabel: consumer2.Label}

	// include a different probability function
	ms2 := make(map[cmptn.OutEdgeStruct]float64)
	ms2[edgeToConsumer2_1] = 0.01
	ms2[edgeToConsumer2_2] = 0.99
	rndparams.AddResponse(src2.Label, RBDataMsg.MsgType, ms2)

	// serialize the initialization struct. N.B., it is not evident here
	// but the CPInitOutputIsYAML flag is ignored by the Serialize method
	// for random functions, and yaml is forced.   Golang lint-picker warns
	// on the json marshalling of this particular response structure, so just
	// avoiding the potential for a run-time error
	rndparamStr, _ := rndparams.Serialize(CPInitOutputIsYAML)
	RBCPInit.AddParam(branch.Label, rndparamStr)

	// when consumer1 gets message of type "decrypted" from "decrypt", respond by doing nothing
	params = cmptn.CreateStaticParameters(RndBranch.CPType, consumer1.Label)
	params.AddResponse(branch.Label, RBConsumeMsg.MsgType, "", "", 0.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	RBCPInit.AddParam(consumer1.Label, paramStr)

	// when consumer2 gets message of type "decrypted" from "decrypt", respond by doing nothing
	params = cmptn.CreateStaticParameters(RndBranch.CPType, consumer2.Label)
	params.AddResponse(branch.Label, RBConsumeMsg.MsgType, "", "", 0.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	RBCPInit.AddParam(consumer2.Label, paramStr)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	RBCPInit.AddParam(consumer2.Label, paramStr)

	// include this CompPattern's initialization informtion into the CP initialization dictionary
	CPInitPrbDict.AddCPInitList(RBCPInit, false)

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
	params = cmptn.CreateStaticParameters(RSAChain.CPType, src.Label)

	// self-initiate, message src -> encrypt every 1.0 second
	params.AddResponse(src.Label, RSAInitMsg.MsgType, RSADataMsg.MsgType, encrypt.Label, 1.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	RSACPInit.AddParam(src.Label, paramStr)

	// when encrypt gets a message of type "data" from source "src", respond by
	// generating a message of type "encrypted" and send to "decrypt"
	params = cmptn.CreateStaticParameters(RSAChain.CPType, encrypt.Label)
	params.AddResponse(src.Label, RSADataMsg.MsgType, RSAEncryptMsg.MsgType, decrypt.Label, 0.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	RSACPInit.AddParam(encrypt.Label, paramStr)

	// when decrypt gets a message of type "encrypted" from "encrypt", respond by
	// generating a message of type "decrypted" and send to function "sink"
	params = cmptn.CreateStaticParameters(RSAChain.CPType, decrypt.Label)
	params.AddResponse(encrypt.Label, RSAEncryptMsg.MsgType, RSADecryptMsg.MsgType, sink.Label, 0.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	RSACPInit.AddParam(decrypt.Label, paramStr)

	// when sink gets message of type "decrypted" from "decrypt", respond by
	// doing nothing
	params = cmptn.CreateStaticParameters(RSAChain.CPType, sink.Label)
	params.AddResponse(decrypt.Label, RSADecryptMsg.MsgType, "", "", 0.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	RSACPInit.AddParam(sink.Label, paramStr)

	// add the RSAChain Initialization to the dictionary
	CPInitPrbDict.AddCPInitList(RSACPInit, false)

	// The structure of the AESChain is identical to RSAChain, and is
	// included only to demonstrate in model building that the AES functions
	// have and use different timing models for their encrypt and decrypt functions.
	// Also, in this example the packet lengths being generated, encrypted, and decrypted are 1/2 the size
	// of RSA, not for any reason other than to demonstrate the possibility of a difference
	AESChain := cmptn.CreateCompPattern("AESChain")

	// the four nodes. Note that function type (e.g., "AES-encrypt") denote general type,
	// while label is an identifier for this CompPattern
	src = cmptn.CreateFunc("generate", "src", "static")
	encrypt = cmptn.CreateFunc("AES-encrypt", "encrypt", "static")
	decrypt = cmptn.CreateFunc("AES-decrypt", "decrypt", "static")
	sink = cmptn.CreateFunc("consume", "sink", "static")

	// add to the CompPattern
	AESChain.AddFunc(src)
	AESChain.AddFunc(encrypt)
	AESChain.AddFunc(decrypt)
	AESChain.AddFunc(sink)

	// include edges.
	AESChain.AddEdge(src.Label, src.Label, "initiate", "initiate")
	AESChain.AddEdge(src.Label, encrypt.Label, "data", "data")
	AESChain.AddEdge(encrypt.Label, decrypt.Label, "encrypted", "encrypted")
	AESChain.AddEdge(decrypt.Label, sink.Label, "decrypted", "encrypted")

	// add the AESChain to the CompPatternDict
	cpPrbDict.AddCompPattern(AESChain, true, false)
	fmt.Println("Added AESChain model to pattern dictionary")

	// Add initialization parameters to AESChain CompPattern
	AESCPInit := cmptn.CreateCPInitList("AESChain", "AESChain", CPInitOutputIsYAML)

	AESInitMsg := cmptn.CreateCompPatternMsg("initiate", 50, 1500)      // self-initiation
	AESDataMsg := cmptn.CreateCompPatternMsg("data", 250, 1500)         // generated pckt, 1/2 size of RSA example
	AESEncryptMsg := cmptn.CreateCompPatternMsg("encrypted", 250, 1500) // encrypted pckt,
	AESDecryptMsg := cmptn.CreateCompPatternMsg("decrypted", 250, 1500) // decrypted pckt for consumption

	AESCPInit.AddMsg(AESInitMsg)
	AESCPInit.AddMsg(AESDataMsg)
	AESCPInit.AddMsg(AESEncryptMsg)
	AESCPInit.AddMsg(AESDecryptMsg)

	// Now add function responses.
	// create parameter block variable for src
	params = cmptn.CreateStaticParameters(AESChain.CPType, src.Label)

	// self-initiate, message src -> encrypt every 1.0 second
	params.AddResponse(src.Label, AESInitMsg.MsgType, AESDataMsg.MsgType, encrypt.Label, 1.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	AESCPInit.AddParam(src.Label, paramStr)

	// when encrypt gets a message of type "data" from source "src", respond by
	// generating a message of type "encrypted" and send to "decrypt"
	params = cmptn.CreateStaticParameters(AESChain.CPType, encrypt.Label)
	params.AddResponse(src.Label, AESDataMsg.MsgType, AESEncryptMsg.MsgType, decrypt.Label, 0.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	AESCPInit.AddParam(encrypt.Label, paramStr)

	// when decrypt gets a message of type "encrypted" from "encrypt", respond by
	// generating a message of type "decrypted" and send to function "sink"
	params = cmptn.CreateStaticParameters(AESChain.CPType, decrypt.Label)
	params.AddResponse(encrypt.Label, AESEncryptMsg.MsgType, AESDecryptMsg.MsgType, sink.Label, 0.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	AESCPInit.AddParam(decrypt.Label, paramStr)

	// when sink gets message of type "decrypted" from "decrypt", respond by
	// doing nothing
	params = cmptn.CreateStaticParameters(AESChain.CPType, sink.Label)
	params.AddResponse(decrypt.Label, AESDecryptMsg.MsgType, "", "", 0.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	AESCPInit.AddParam(sink.Label, paramStr)

	// add the AESChain Initialization to the dictionary
	CPInitPrbDict.AddCPInitList(AESCPInit, false)

	// Add a client-server CompPattern w/o certificates
	// Just two functions, the client (self-initiating) and server
	SimpleWeb := cmptn.CreateCompPattern("SimpleWeb")

	// two funcs
	client := cmptn.CreateFunc("generate", "client", "static")
	server := cmptn.CreateFunc("serve", "server", "static")
	SimpleWeb.AddFunc(client)
	SimpleWeb.AddFunc(server)

	// three edges: one to self-initiate, one to request service, the other to report service provided
	SimpleWeb.AddEdge(client.Label, client.Label, "initiate", "initiate")
	SimpleWeb.AddEdge(client.Label, server.Label, "request", "request")
	SimpleWeb.AddEdge(server.Label, client.Label, "response", "response")

	// initialization structure for SimpleWeb
	SimpleWebCPInit := cmptn.CreateCPInitList("SimpleWeb", "SimpleWeb", CPInitOutputIsYAML)

	// three message types
	SimpleWebInitMsg := cmptn.CreateCompPatternMsg("initiate", 100, 1500)
	SimpleWebReqMsg := cmptn.CreateCompPatternMsg("request", 250, 1500)
	SimpleWebRspMsg := cmptn.CreateCompPatternMsg("response", 1500, 1500)
	SimpleWebCPInit.AddMsg(SimpleWebInitMsg)
	SimpleWebCPInit.AddMsg(SimpleWebReqMsg)
	SimpleWebCPInit.AddMsg(SimpleWebRspMsg)

	// when "client" gets message of type "initiate" from self, respond by
	// sending "request" message to "server".  Every 3.333 seconds...
	params = cmptn.CreateStaticParameters(SimpleWeb.CPType, client.Label)
	params.AddResponse(client.Label, SimpleWebInitMsg.MsgType, SimpleWebReqMsg.MsgType, server.Label, 3.333)

	// notice that client needs to respond to messages from server also...do nothing
	params.AddResponse(server.Label, SimpleWebRspMsg.MsgType, "", "", 0.0)

	// save the server initialization structure
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	SimpleWebCPInit.AddParam(client.Label, paramStr)

	// when "server" gets message of type "request" from "client", respond by
	// sending "response" message to "client"
	params = cmptn.CreateStaticParameters(SimpleWeb.CPType, server.Label)
	params.AddResponse(client.Label, SimpleWebReqMsg.MsgType, SimpleWebRspMsg.MsgType, client.Label, 0.0)

	// just one response to save, so serialize the func's initiationization struct
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	SimpleWebCPInit.AddParam(server.Label, paramStr)

	// all the functions have their initialization structs built and committed, put
	// the SimpleWeb wrapper into the dictionary
	CPInitPrbDict.AddCPInitList(SimpleWebCPInit, false)

	// add the CompPattern topology to the dictionary
	cpPrbDict.AddCompPattern(SimpleWeb, true, false)
	fmt.Println("Added client-server model to pattern dictionary")

	// Add a client-server pattern with simple certificate serving.
	// Four functions: client, server, pki-cache, pki-server
	PKIWeb := cmptn.CreateCompPattern("PKIWeb")
	client = cmptn.CreateFunc("generate", "web-client", "static")
	server = cmptn.CreateFunc("pki-server", "web-server", "stateful")
	cache := cmptn.CreateFunc("pki-cache", "cache", "stateful")
	certSrvr := cmptn.CreateFunc("cert-server", "certs", "static")

	PKIWeb.AddFunc(client)
	PKIWeb.AddFunc(server)
	PKIWeb.AddFunc(cache)
	PKIWeb.AddFunc(certSrvr)

	PKIWeb.AddEdge(client.Label, client.Label, "initiate", "initiate")
	PKIWeb.AddEdge(client.Label, server.Label, "request", "web-request")
	PKIWeb.AddEdge(server.Label, cache.Label, "request", "cert-request")
	PKIWeb.AddEdge(cache.Label, certSrvr.Label, "request", "cert-request")
	PKIWeb.AddEdge(certSrvr.Label, cache.Label, "response", "cert-response")
	PKIWeb.AddEdge(cache.Label, server.Label, "response", "cert-response")
	PKIWeb.AddEdge(server.Label, client.Label, "response", "web-response")

	// add the PKIWeb to the CompPatternDict
	cpPrbDict.AddCompPattern(PKIWeb, true, false)
	fmt.Println("Added PKIWeb with certs model to pattern dictionary")

	// generate the event to generate a request
	PKIWebInitMsg := cmptn.CreateCompPatternMsg("initiate", 100, 1500)

	// client sends request to server
	PKIWebReqMsg := cmptn.CreateCompPatternMsg("request", 250, 1500)

	// server sends response back
	PKIWebRspMsg := cmptn.CreateCompPatternMsg("response", 1500, 1500)

	// server asks cache for a cert
	PKIWebCacheReqMsg := cmptn.CreateCompPatternMsg("request", 1500, 1500)

	// cache returns a response to the server
	PKIWebCacheRspMsg := cmptn.CreateCompPatternMsg("response", 1500, 1500)

	// cache requests cert from pki cert server
	PKIWebCertReqMsg := cmptn.CreateCompPatternMsg("request", 1500, 1500)

	// pki cert sends a response to the cache
	PKIWebCertRspMsg := cmptn.CreateCompPatternMsg("response", 1500, 1500)

	// create the CompPattern's initialization structure and stick in the messages
	PKIWebCPInit := cmptn.CreateCPInitList("PKIWeb", "PKIWeb", CPInitOutputIsYAML)
	PKIWebCPInit.AddMsg(PKIWebInitMsg)
	PKIWebCPInit.AddMsg(PKIWebReqMsg)
	PKIWebCPInit.AddMsg(PKIWebRspMsg)
	PKIWebCPInit.AddMsg(PKIWebCacheReqMsg)
	PKIWebCPInit.AddMsg(PKIWebCacheRspMsg)
	PKIWebCPInit.AddMsg(PKIWebCertReqMsg)
	PKIWebCPInit.AddMsg(PKIWebCertRspMsg)

	// initialize the client.
	params = cmptn.CreateStaticParameters(PKIWeb.CPType, client.Label)

	// client response to initiate message, self-initiate every 1.5 seconds
	params.AddResponse(client.Label, PKIWebInitMsg.MsgType, PKIWebReqMsg.MsgType, server.Label, 1.5)

	// client response to returned response from server
	params.AddResponse(server.Label, PKIWebRspMsg.MsgType, "", "", 0.0)
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)

	// save the clients's initialization
	PKIWebCPInit.AddParam(client.Label, paramStr)

	// create the initialization block for the server
	// state block holds threshold "cach-interval" dictating how often to forward requests
	// to cache, and "visits" which is the number times requests have been seen coming from the client
	state := make(map[string]string)
	state["threshold"] = "1000" // forward every 1000th request
	state["visits"] = "0"       // number of visits to this function

	// The stateful func responds using code identified by keyword 'filter'
	statefulparams := cmptn.CreateStatefulParameters(PKIWeb.CPType, server.Label, "filter", state)

	// one response is directly back to the client
	statefulparams.AddResponse(client.Label, PKIWebReqMsg.MsgType, PKIWebRspMsg.MsgType, client.Label, 0.0)

	// one response forwards a request to the cache.  This response is marked to distinguish from the other in the response selection logic
	statefulparams.AddResponse(client.Label, PKIWebReqMsg.MsgType, PKIWebCacheReqMsg.MsgType, cache.Label, 0.0)

	// one response transforms a response from the cache on to the client
	statefulparams.AddResponse(cache.Label, PKIWebRspMsg.MsgType, PKIWebCacheRspMsg.MsgType, client.Label, 0.0)

	// bundle up the initialization structure and save it for this CompPattern
	statefulparamStr, _ := statefulparams.Serialize(CPInitOutputIsYAML)
	PKIWebCPInit.AddParam(server.Label, statefulparamStr)

	// now the cache
	// every 100th request to the cache gets forwarded to the pki-server
	state = make(map[string]string)
	state["threshold"] = "100"
	state["visits"] = "0"

	// create an initialization block for the cache.  We can use the same logic for deciding whether to forward the request or not,
	// and the same label on the edge we'd use for that selection.  Topologically identical, changing only the funcs labeled
	statefulparams = cmptn.CreateStatefulParameters(PKIWeb.CPType, cache.Label, "filter", state)
	statefulparams.AddResponse(server.Label, PKIWebCacheReqMsg.MsgType, PKIWebCacheRspMsg.MsgType, server.Label, 1.0)
	statefulparams.AddResponse(server.Label, PKIWebCacheReqMsg.MsgType, PKIWebCertReqMsg.MsgType, certSrvr.Label, 1.0)
	statefulparams.AddResponse(certSrvr.Label, PKIWebRspMsg.MsgType, PKIWebCacheRspMsg.MsgType, server.Label, 0.0)
	statefulparamStr, _ = statefulparams.Serialize(CPInitOutputIsYAML)
	PKIWebCPInit.AddParam(cache.Label, statefulparamStr)

	// initialization for the certSrvr
	params = cmptn.CreateStaticParameters(PKIWeb.CPType, certSrvr.Label)
	// just one reponse possible, return result
	params.AddResponse(cache.Label, PKIWebCertReqMsg.MsgType, PKIWebCertRspMsg.MsgType, cache.Label, 0.0)

	// wrap up the certSrvr responses
	paramStr, _ = params.Serialize(CPInitOutputIsYAML)
	PKIWebCPInit.AddParam(certSrvr.Label, paramStr)

	// put the PKIWeb initialization structure into the initialization dictionary
	CPInitPrbDict.AddCPInitList(PKIWebCPInit, false)

	// write the computation patterns out to file
	cpPrbDict.WriteToFile(cpPrbFile)

	// write the initialization out to file
	CPInitPrbDict.WriteToFile(CPInitPrbFile)

	fmt.Printf("Output files %s, %s written!\n", cpPrbFile, CPInitPrbFile)
}
