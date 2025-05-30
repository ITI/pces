package mrnes

import (
	// "code.iti.illinois.edu/dpss/simulator/desc"
	"github.com/iti/evt/evtm"
	"code.iti.illinois.edu/dpss/simulator/vrtime"
	_ "errors"
	"fmt"
	"log"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/layers"
	"github.com/iti/rngstream"
	"os/exec"
	_ "log"
	"strings"
	"strconv"
	"time"
	"os"
	"io"	
	"net"
	"net/netip"
	"bufio"
	"path/filepath"
)

var evtTrace bool = false
var pcktTrace bool = false
var pgTrace bool = false
var printTrace bool = false

var printErr bool = true
var printFlowTrace = false
var printEvtTrace = false
var printInfo = false

var remapIP map[string]string = make(map[string]string)

var simHwAddr map[string]net.HardwareAddr = make(map[string]net.HardwareAddr)
var emuHwAddr map[string]net.HardwareAddr = make(map[string]net.HardwareAddr)

var deviceToPcapNode map[string]*pcapNode = make(map[string]*pcapNode)


var numPckts uint = 0
func nxtPcktid() uint {
	numPckts += 1
	return numPckts
}

// core node structure for pcap packets read from file
// with a randomly selected intrArrival time between them
type pcapNode struct {
	srcName	   string		// name of the device (interface) that produces packets
	simIPAddr string
	emuIPAddr string
	simHwAddr net.HardwareAddr		// MAC address, as reported by the net module for this device
	emuHwAddr net.HardwareAddr		// MAC address, as reported by the net module for this device
	endptName	string		// name of the endpoint holding this protocol graph node, for logging purposes
	active     bool			// flag indicating whether this node is actively acquiring packets
	nxtEvtId   int   // id of next packet generation		// saved in case needed to cancel the scheduled event
	nxtEvtTime int64 // time of next packet generation

	online	bool	
	intrArrival   *RandomMeasure // parameters for random number generator to sample packet inter-arrival times
	msrcnvt       msrCnvt		 // parameter for inter-arrival time generation
	handle	 	  *pcap.Handle   // when we create the packet source we get a handle used for reads and writes
	pcktSrc		  *gopacket.PacketSource // 

	pcktsPushed   int			 // count the number of packets pushed back at the interface
	pcktsPulled   int			// count the number of packets recv'd from the source
	cg            *chanGuard	// structure for moving packets and commands between producer and consumer

	// variables needed only by pcnd represenation
	script		  string		// path name of script file which when run creates virtual interface
	stdin		io.WriteCloser	// for sending things to the script through a stdin pipe
	stdout		io.ReadCloser	// for getting things from the script through a stdout pipe
	srcBuildTimeout float64	// when >0, the number of seconds we allow for no activity on the interface named by srcName
	pollReader	bool			// switch to build device and be prepared to run it, but don't run it.  For scalability studies
								// placed here to allow further finer-grained flow control
	fullARP bool				// when set we pass ARP requests through to do the ARP search thing out in the simulator.
								// Fleshed out this will support simulation of ARP poisoning attacks
}

func reportEvt(evtmgr *evtm.EventManager, funcName, objName, msg string) {
	if !(evtTrace || pcktTrace) {
		return
	}
	fmt.Printf("At %f function %s for %s reports %s\n", evtmgr.CurrentSeconds(), funcName, objName, msg)
	if logging {
		log.Printf("At %f function %s for %s reports %s\n", evtmgr.CurrentSeconds(), funcName, objName, msg)
	}
}

// createPcapNode is the constructor for pcapNode
func createPcapNode() *pcapNode {
	// allocate space and include the core, create the IP remapping table
	return new(pcapNode)
}

// makeARPReply takes a packet already recognized as being ARP and expected
// to be an ARP request, and creates from it an ARP reply that makes
// the argument hardware address the destination that is sought
//
func (pcnd *pcapNode) makeARPReply(pckt *gopacket.Packet, dstIP string) ([]byte, bool) {
	arpLayer := (*pckt).Layer(layers.LayerTypeARP)
	if arpLayer == nil {
		return nil, false
	}
	arp_in := arpLayer.(*layers.ARP)

	// if the ARP operation is not a request, don't reflect it
    if arp_in.Operation != layers.ARPRequest {
		return nil, false
	}

	// the fields of interest are
	// arp.SourceHwAddress
	// arp.SourceProtAddress
	// arp.DstHwAddress
	// arp.DstProtAddress
	// arp.Operation
	//   From these create a packet of a response

    // Set up all the layers' fields we can.
	eth := layers.Ethernet{
		SrcMAC:       emuHwAddr[dstIP],
		DstMAC:       arp_in.SourceHwAddress,
		EthernetType: layers.EthernetTypeARP,
		}

    arp := layers.ARP{
        AddrType:			layers.LinkTypeEthernet,
        Protocol:			layers.EthernetTypeIPv4,
        HwAddressSize:		6,
        ProtAddressSize:	4,
        Operation:			layers.ARPReply,
        SourceHwAddress:	emuHwAddr[dstIP],
        SourceProtAddress:	arp_in.DstProtAddress,
        DstHwAddress:		arp_in.SourceHwAddress, 
		DstProtAddress:		arp_in.SourceProtAddress,
	}

    // Set up buffer and options for serialization.
    buf := gopacket.NewSerializeBuffer()
    opts := gopacket.SerializeOptions{
        FixLengths:       true,
        ComputeChecksums: true,
    }

	gopacket.SerializeLayers(buf, opts, &eth, &arp)
	return buf.Bytes(), true
}

func pullPcktsFromDevice(pcnd *pcapNode) {
	evtmgr := pcnd.EvtMgr()

	// each iteration of this loop acquires a packet, computes an arrival delay,
	// and schedules an event to process the packet
	for {
		// block until a 'start' command arrives to request the next packet
		cmd := <-pcnd.cg.cmd
		if cmd != start {
			log.Printf("abnormal termination on source trigger %d\n", cmd)
			fmt.Printf("abnormal termination on source trigger %d\n", cmd)
			// anything other than a 'start' command is taken to be terminate
			return
		}
		if pcktTrace { 
			fmt.Printf("At time %f %s is released to acquire another packet\n", evtmgr.CurrentSeconds(),
				pcnd.endptName)
			log.Printf("At time %f %s is released to acquire another packet\n", evtmgr.CurrentSeconds(),
				pcnd.endptName)
			}
		// block until data is available		
		pckt, openChan := <-pcnd.cg.data 

		if pcktTrace {
			fmt.Printf("%s acquires pckt %d nil=%t, openChan=%t\n",
				pcnd.cg.ownerName, pcnd.pcktsPulled, (pckt==nil), openChan)
			log.Printf("%s acquires pckt %d nil=%t, openChan=%t\n",
				pcnd.cg.ownerName, pcnd.pcktsPulled, (pckt==nil), openChan)
			}
		pcnd.pcktsPulled += 1

		if pcktTrace {
			fmt.Printf("%v\n", pcktToBytes(&pckt))
			log.Printf("%v\n", pcktToBytes(&pckt))
		}

		// non-nil packet means we schedule its arrival in the future
		if pckt != nil {

			// sample the delay in seconds 
			delayTimeInSeconds := pcnd.intrArrival.Sample()
			// convert to ticks
			intrArrivalTimeInTicks := vrtime.SecondsToTicks(delayTimeInSeconds)

			_, nxtTime := evtmgr.Schedule(pcnd, pckt, processPcktFromDevice,
				vrtime.CreateTime(intrArrivalTimeInTicks, evtmgr.Ordering()))

			if pcktTrace {
				fmt.Printf("At %f %s schedules pckt processing to occur at %f\n",
					evtmgr.CurrentSeconds(), pcnd.endptName, nxtTime.Seconds()) 
				log.Printf("At %f %s schedules pckt processing to occur at %f\n",
					evtmgr.CurrentSeconds(), pcnd.endptName, nxtTime.Seconds()) 
				}
				// break out of loop so that we don't hit cg.data again without a prompt
			continue
		}

		if pckt == nil && !openChan {
			fmt.Println("Leaving on observation of closed channel")
			log.Println("Leaving on observation of closed channel")
			break
		}
   }
}

// initialize required for protocolNode.  Is passed a struct
// created at model build time to initialize the node.
// initialize copies parameters included in this structure
func (pcnd *pcapNode) initialize(args ...any) (any, error) {

	// TODO create PcapCfg in 
	// cast the trace description from args[0]
	pcapDesc := args[0].(desc.PcapCfg)

	// if the configuration says to ignore this one, ignore it immediately!
	if !pcapDesc.Active {
		return nil, nil
	}

	// copy parameters placed in the PcapCfg struct into the pcapNode struct
	pcnd.active = pcapDesc.Active					// this source is active
	pcnd.srcBuildTimeout = pcapDesc.SrcBuildTimeout // when we create the source through a script execution,
														// we have to wait until the OS reflects the virtual interface
														// this is created.  srcBuildTimeout is how long to wait
	pcnd.handle = nil									// holds the handle we use to read-from and write-to the source 
	pcnd.online = pcapDesc.Online						// interacting with an online interface, not a file
														// device (or script behind it)

	pcnd.srcName = pcapDesc.SrcName					// the device name 
	deviceToPcapNode[pcnd.srcName] = pcnd
	pcnd.simIPAddr = pcapDesc.SimIPAddr
	pcnd.emuIPAddr = pcapDesc.EmuIPAddr

	pcnd.endptName = pcapDesc.EndptName				// name of the endpoint onto which this node is mapped
	pcnd.fullARP = pcapDesc.FullARP

	if printTrace { 
		fmt.Printf("--------pcapDesc: %#v------------\n", pcapDesc) // show what we've got, for tracing
	}
	pcnd.pcktsPushed = 0								// keep track of the number of packets we've received
	pcnd.pcktsPulled = 0								// keep track of the number of packets we've received


	// if the runtime line identified a directory where source startup scripts are kept,
	// create the absolute path name for that directory.  Otherwise ensure that
	// the script path name is empty, which serves as a flag that we aren't using scripts we created
	if len(scriptsDir) > 0 {
		pcnd.script = filepath.Join(scriptsDir, pcapDesc.Script)
	} else {
		pcnd.script = ""
	}

	// Thinking of the packets as jobs arriving to a queue, we model the (random) inter-arrival time,
	// which for us means the time between successive requests
	intrArrival := CreateRandomMeasure(pcapDesc.OffMeasureDist, pcapDesc.OffMeasureDistParams, pcnd.rns())
	pcnd.intrArrival = intrArrival
	pcnd.msrcnvt = msrCnvt{workloadMaxRate: pcapDesc.OffWorkloadRateMax, 
		workloadCommRatio: float64(1.0), measureType: pcapDesc.OffMeasureType}

	// register interest in ALL carryPcap packets.
	children := pcnd.children()
	child := children[0]

	// use -1 as code in port attribute to signal accepting all packets.
	// First create the service direction structure
	srvIdx := serviceIdx{cmdType: carryPcap, port: -1}
	pgnFunctionMsgVar := pgnFunctionMsg{funcType: registerService, service: srvIdx}
	pgnMsgVar := pgnMsg{msgType: carryFunction, connType: pt2pt, msg: pgnFunctionMsgVar}

	// have the child process the message to register this service code
	child.fromAbove(pcnd, "port", &pgnMsgVar)

	// set up the packet source (if scripted), handle and channel
	initAccessToDevice(pcnd)

	// fire up the goroutine that grabs packets and schedules processing
	go pullPcktsFromDevice(pcnd) 

	return 0, nil
}

func (pcnd *pcapNode) start() {
	pcnd.cg.cmd <- start
}

// fromBelow delivers a packet that has passed through up the stack.
// if it is a pcap message (and it should be, but check) we
// push it out to the attached device
//
func (pcnd *pcapNode) fromBelow(child protocolNode, context any, pgnMsgVar *pgnMsg) (any, int, error) {
	evtmgr := pcnd.EvtMgr()

		if pgTrace { 
			fmt.Printf("pcap fromBelow carryPcap %t\n", pgnMsgVar.msgType == carryPcap)
			log.Printf("pcap fromBelow carryPcap %t\n", pgnMsgVar.msgType == carryPcap)
		}

		if pgnMsgVar.msgType == carryPcap {
			pcapMsg := pgnMsgVar.msg.(pgnPcapMsg)

			// remember that what is carried here is a pointer to a gopacket.Packet
			packet := pcapMsg.pckt
			if pgTrace {
				fmt.Printf("at time %f %s pushes packet %d up to device, %v\n", 
					evtmgr.CurrentSeconds(), pcnd.endptName, pcnd.pcktsPushed, *packet)
				log.Printf("at time %f %s pushes packet %d up to device, %v\n", 
					evtmgr.CurrentSeconds(), pcnd.endptName, pcnd.pcktsPushed, *packet)
				}	
			pcnd.pcktsPushed += 1
			err := pcnd.writePcktToDevice(packet)
			return nil, pcnd.id(), err
		}
		// silent return
		if pgTrace {
			fmt.Println("pcap fromBelow exits w/o push")
			log.Println("pcap fromBelow exits w/o push")
		}

		return nil, pcnd.id(), nil
}

func pcktToBytes(packet *gopacket.Packet) []byte {
		ethLayer := (*packet).Layer(layers.LayerTypeEthernet)

		eth := ethLayer.(*layers.Ethernet)
		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{}
		_ = gopacket.SerializeLayers(buf, opts, eth, gopacket.Payload(eth.Payload))
		return buf.Bytes()
	}


// writePcktToDevice transforms a gopacket Packet into a slice of bytes
// that are written to the device
//
func (pcnd *pcapNode) writePcktToDevice(packet *gopacket.Packet) error {
		ethLayer := (*packet).Layer(layers.LayerTypeEthernet)
		if ethLayer == nil {
			log.Println("ethLayer == nil")
			return fmt.Errorf("ethLayer == nil")
		}

		eth := ethLayer.(*layers.Ethernet)
		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{}
		err := gopacket.SerializeLayers(buf, opts, eth, gopacket.Payload(eth.Payload))
		if err != nil {
			log.Println(err)
			return fmt.Errorf("problem serializing packet %s\n", err)
		}

		outPacket := buf.Bytes()
		err = pcnd.handle.WritePacketData(outPacket)
		if err != nil {
			log.Println(err)
			return fmt.Errorf("problem writing packet %s\n", err)
		}
		if pcktTrace {
			fmt.Printf("Forwarded packet %v\n",outPacket) 
			log.Printf("Forwarded packet %v\n",outPacket) 
		}
		return nil
	}

func stopPcap(evtmgr *evtm.EventManager, pcapArg any, args any) any {
	pcnd := pcapArg.(*pcapNode)
	evtmgr.RemoveEvent(pcnd.nxtEvtId)
	return true
}

func pcapCleanup(evtmgr *evtm.EventManager, pcapArg any, args any) any {

	pcnd := pcapArg.(*pcapNode)

	if evtTrace {
		reportEvt(evtmgr, "pcapCleanup", pcnd.endptName, "enters, signals ack")
	}

	// do something

	return nil
}

func harvestEmuHwAddr(pcnd *pcapNode) {
	if !pcnd.online {
		return
	}

	// gather all the files /tmp/mrnes/trace.*
	traceFileName := fmt.Sprintf("/tmp/mrnes/trace.%s", pcnd.srcName)

	tfs, tf_err := os.ReadFile(traceFileName)
	if tf_err != nil {
		fmt.Printf("Problem opening trace file %s, %s\n", traceFileName, tf_err)
	} else {
		// split the trace file into lines
		words := strings.Fields(string(tfs))

		var hwIdx int = -1
		for idx, word := range words {
			if word == "link/ether" {
				hwIdx = idx+1
				break
			}
		}
		if hwIdx == -1 {
			fmt.Printf("Problem parsing trace file %s\n", traceFileName)
			log.Printf("Problem parsing trace file %s\n", traceFileName)
		} else {
			var p_err error
			pcnd.emuHwAddr, p_err = net.ParseMAC(words[hwIdx])
			if p_err != nil {
				fmt.Printf("Problem parsing trace file %s\n", traceFileName)
				log.Printf("Problem parsing trace file %s\n", traceFileName)
			} else {
				// save the mac address indexed by the associated IP address
				emuHwAddr[pcnd.emuIPAddr] = pcnd.emuHwAddr
			}
		}
	}
}

func initAccessToDevice(pcnd *pcapNode) {

	var useScript bool = false		// flags whether the live source is started 
									// from a shell script we cause to execute
	var who string					// identifier used when logging traces
	var err error
	var timedOut bool = false

	// initialization depends on the source 
	switch pcnd.online {
		case true:

			// variables themselves used for either type of source, we know here to get them from onlineNode
			who = pcnd.endptName // used in logging reports

			// pcnd.script will be non-empty when at configuration we are told that this source
			// is brought to existence through the execution of a shell script.  In that case
			// we use the exec module to set up the execution and setup pipes for stdin 
			// for communicating with the process being executed.  Typically (but not required)
			// the script brings up a bash shell that will need a command feed to it to generate the traffic source)
			//
			if pcnd.script != "" {
				if pcktTrace {
					fmt.Printf("initializing with script %s\n", pcnd.script)
				}

				// create the data structure describing this as-yet-unstarted execution
				// the assumption is that pcnd.script is a vetted path name for
				// a script that can be executed by call (e.g. has #!/usr/bin sh as the first line
				// and has execution permission)
				//
				cmd := exec.Command(pcnd.script)		

				// set up the pipe for stdin
				var in_err, out_err error
				pcnd.stdin, in_err = cmd.StdinPipe()
				if in_err != nil {
					fmt.Printf("In %s fatal error setting up stdin pipe", who)
					log.Printf("In %s fatal error setting up stdin pipe", who)
					os.Exit(1)
				}	
				if pcnd.stdin == nil {
					fmt.Printf("In %s fatal error stdin pipe is nil", who)
					log.Printf("In %s fatal error stdin pipe is nil", who)
					os.Exit(1)
				}

				pcnd.stdout, out_err = cmd.StdoutPipe()
				if out_err != nil {
					fmt.Printf("In %s fatal error setting up stdout pipe", who)
					log.Printf("In %s fatal error setting up stdout pipe", who)
					os.Exit(1)
				}	
				if pcnd.stdout == nil {
					fmt.Printf("In %s fatal error stdout pipe is nil", who)
					log.Printf("In %s fatal error stdout pipe is nil", who)
					os.Exit(1)
				}

				// execute the script
				srt_err := cmd.Start()

				// look at the error messages coming back
				if srt_err != nil {
					log.Printf("In %s cmd.Start returns err %s\n", who, srt_err)
					log.Fatal(srt_err)
				} else if cmdTrace {
					fmt.Printf("In %s cmd.Start successfully executes\n", who )
					log.Printf("In %s cmd.Start successfully executes\n", who )
				}

				// read from stdout until we see the echo 'Done'
				scanner := bufio.NewScanner(pcnd.stdout)
				startScan := time.Now()
				for scanner.Scan() {
					text := scanner.Text()
					if strings.Contains(text,"Done!") {
						break
					}
					finish :=  time.Now()
					timedOut := time.Duration(2*time.Second*1e9) < finish.Sub(startScan)
					if timedOut {
						fmt.Println("Unable to launch shell for %s\n", pcnd.srcName)
						os.Exit(1)
					}
				}

				// time.Sleep(500*time.Millisecond)
				useScript = true
			}

			// open the connection to the (possibly new) device,
			// if we used a script to create it we wait for no more
			// than pcnd.srcBuildTimeout seconds
			// timeout interface is in seconds, gap in times is in nanoseconds.
			// If we aren't using a script, an error is fatal
			//
			timeoutInNanoSecs := int64(pcnd.srcBuildTimeout*1e+9)

			// if we aren't waiting for the device to come up we'll
			// exit the loop after 1 iteration.  Otherwise we exit
			// either when the device is recognized or we time out
			//
			startTimer := time.Now()
			timedOut = false
			for pcnd.handle == nil && !timedOut {
				pcnd.handle, err = pcap.OpenLive(pcnd.srcName, 65535, true, pcap.BlockForever)
				//pcnd.handle, err = pcap.OpenLive(pcnd.srcName, 65535, true, time.Microsecond*2)

				if err != nil || pcnd.handle == nil {
					// open error.  If we are waiting for the device to appear we expect this
					// and back off.  We are aren't then this is a fatal error
					//
					if !useScript {
						fmt.Printf("error %s on OpenLive(%s)\n", err, pcnd.srcName)
						log.Printf("error %s on OpenLive(%s)\n", err, pcnd.srcName)
						os.Exit(1)
					} else {
						// back off for a little time
						finish :=  time.Now()
						timedOut = time.Duration(timeoutInNanoSecs) < finish.Sub(startTimer)
						if !timedOut {
							time.Sleep(100*time.Millisecond)
						}
					}
					err = nil
					pcnd.handle = nil
					continue
				}
			}

			// if it is still the case that the pcap handle is nil, that means
			// that we tried to use a script to create the virtual interface
			// but after repeated attempts that led to a time-out, we did not,
			// which constitutes an error
			//
			if pcnd.handle == nil {
				fmt.Printf("error on OpenLive(%s)\n", pcnd.srcName)
				log.Printf("error on OpenLive(%s)\n", pcnd.srcName)
				os.Exit(1)
			}

			// the virtual interface is established and some MAC address was given to it.
			// Save that for ARP stuff
			//
			virtIntrfc, _ := net.InterfaceByName(pcnd.srcName)
			pcnd.simHwAddr = virtIntrfc.HardwareAddr

		case false :
			pcnd.handle, err = pcap.OpenOffline(pcnd.srcName) 
			if err != nil {
				fmt.Printf("error on OpenOffline(%s)\n", pcnd.srcName)
				log.Printf("error on OpenOffline(%s)\n", pcnd.srcName)
				os.Exit(1)
			}
	}

	// source type dependency stuff mostly finished now

	// by default we are NOT doing lazy decoding and ARE doing copy decoding
	// so the packets coming from src will be concurrency safe
	src := gopacket.NewPacketSource(pcnd.handle, layers.LayerTypeEthernet)

	var in chan gopacket.Packet

	pcnd.cg = new(chanGuard)
	pcnd.cg.ownerName = pcnd.endptName
	pcnd.cg.data = src.Packets()
	pcnd.cg.cmd = make(chan chanCmd,1)

	if !pcnd.online {
		// the packets from a file will always be available.  To better 
		// create behaviour similar to when the interface is live, we
		// artificially include a real-time 1 second delay between successive
		// reports of packets
		//
		in = make(chan gopacket.Packet)
		go func(chOut chan gopacket.Packet, inSrc chan gopacket.Packet) {
			for {
				time.Sleep(2*time.Second)
				pckt := <- inSrc
				chOut <- pckt
			}
		}(in, pcnd.cg.data)
		pcnd.cg.data = in
	}
}

// processPcktFromDevice is scheduled upon the receipt of a packet from the device
// Release the source to grab another packet. Possibly respond to an ARP request.
// Format the wrapper for the pckt to pass through the protocol graph, and 
// push it to the transport layer
//
func processPcktFromDevice(evtmgr *evtm.EventManager, pcndVar any, pcktVar any) any {
	pcnd := pcndVar.(*pcapNode)
	pckt := pcktVar.(gopacket.Packet)

	if pcktTrace {
		fmt.Printf("On %s process packet %v\n", pcnd.endptName, pckt)
		log.Printf("On %s process packet %v\n", pcnd.endptName, pckt)
	}

	// if the packet we got is an ARP request and we aren't doing full
	// ARP, schedule a push to the device
	//
	// now push the received packet down the protocol graph
	isIPv4, protocol, srcIP, srcPort, tstDstIP, dstPort := ExtractIPv4Hdr(&pckt)

	// bail immediately on non ipv4 messages
	if !isIPv4 {
		pcnd.cg.cmd <- start
		return 0
	}

	var tgtIPbytes []byte
	if protocol=="ARP" {
		tgtIPbytes = ExtractARPTgtIP(&pckt)
		adrs := make([]string,4)
		adrs[0] = strconv.Itoa(int(tgtIPbytes[0]))
		adrs[1] = strconv.Itoa(int(tgtIPbytes[1]))
		adrs[2] = strconv.Itoa(int(tgtIPbytes[2]))
		adrs[3] = strconv.Itoa(int(tgtIPbytes[3]))
		tstDstIP = strings.Join(adrs,".")
	} else {
		_, parse_err := netip.ParseAddr(tstDstIP)
		if parse_err != nil {
			fmt.Printf("unrecognized dstIP, protocol --%s--\n", protocol)
			pcnd.cg.cmd <- start
			return 0
		} 
	}

	// if the message is an ARP request and we are not suppressing 'quickARP' 
	// create a response and send it back

	if protocol=="ARP" && !pcnd.fullARP {
		if pcktTrace {
			fmt.Printf("%s flips ARP\n", pcnd.endptName)
			log.Printf("%s flips ARP\n", pcnd.endptName)
		}
		packetBytes, reflected := pcnd.makeARPReply(&pckt, tstDstIP)

		// if we were successful in creating the reflection, write it directly
		if reflected {

			pcnd.handle.WritePacketData(packetBytes)

			// release the packet acquisition goroutine to grab another
			if pcktTrace {
				fmt.Printf("at time %f %s releases source for another packet\n",
					evtmgr.CurrentSeconds(), pcnd.endptName)
				log.Printf("at time %f %s releases source for another packet\n",
					evtmgr.CurrentSeconds(), pcnd.endptName)
			}

			pcnd.cg.cmd <- start
			return 0
		}
		// otherwise leave it as is and push down the protocol graph
	}

	// push the packet down the protocol graph
	var dstIP string
	var dstPresent bool
	var defaultPresent bool

	_, parse_err := netip.ParseAddr(tstDstIP)
	if parse_err == nil {
		// set dstIP to be the destination IP address as viewed within the simulator.
		// one thing could happen is that all IPs get mapped to one.
		dstIP, defaultPresent = remapIP["*"]
		if !defaultPresent {
			// wildcard not present, so see if the offered destination IP is in the table
			dstIP, dstPresent = remapIP[tstDstIP]
			if !dstPresent {
				fmt.Printf("failed remap of %s, packet dropped\n", tstDstIP)
				log.Printf("failed remap of %s, packet dropped\n", tstDstIP)
				// no remapping information stored, so dropped 
				pcnd.cg.cmd <- start
				return 0
			} else {
				if pgTrace {
					fmt.Printf("remap %s -> %s\n", tstDstIP, dstIP)
					log.Printf("remap %s -> %s\n", tstDstIP, dstIP)
				}
			}
		}
	}

	// routing is done by object identity, not by IP, so we get the ids of the
	// source and destination
	// We know that the id of the src must be that of the hosting endpoint
	srcId := pcnd.endpoint().id()

	// the dstIP is an IP address of some interface in some NIC attached
	// to some endpoint.  The id of that endpoint is what we want.
	// In the course of building the topology we save the binding of
	// NIC IP address to endpoint
	//
	endpt, present := endpointByIP[dstIP]
	if !present {
		fmt.Printf("endpt unspecified for dstIP %s", dstIP)
		log.Printf("endpt unspecified for dstIP %s", dstIP)
		return fmt.Errorf("endpt unspecified for dstIP %s", dstIP)
	}
	dstId := endpt.id()

	// make the struct that carries the description of the packet to be pushed
	adrsHdr := adrsHeader{protocol: protocol, srcId: srcId, dstId: dstId,
		srcIP: srcIP, srcPort: srcPort, dstIP: dstIP, dstPort: dstPort}

	// push the packet through to the next layer.  The type
	// of the pgnMsg is pgnPcapMsg
	//
	pgnPcapMsg := pgnPcapMsg{
		msgType:  carryPcap,
		connType: pt2pt,
		pcktId:   nxtPcktid(),
		adrsHdr:  adrsHdr,
		pckt:     &pckt}

	pgnMsgVar := pgnMsg{
		msgType:  carryPcap,
		connType: pt2pt,
		adrsHdr:  adrsHdr,
		msg:      pgnPcapMsg}

	transportLayers, err := commonChildrenFromLabel(pcnd, "transport")
	if err != nil {
		fmt.Println("oops")
		log.Println("oops")
		return 0
	}
	// push this down, to the IP translation layer
	if pgTrace {
		msg := fmt.Sprintf("pushes received packet down to transport layer")
		reportEvt(evtmgr, "processPcktFromDevice", pcnd.endptName, msg)
	}
	transportLayers[0].fromAbove(pcnd, pcnd.id(), &pgnMsgVar)

	// eventually control comes back

	if pgTrace {
		msg := fmt.Sprintf("returned from push down protocol graph")
		reportEvt(evtmgr, "processPcktFromDevice", pcnd.endptName, msg)
	}

	// release the packet acquisition goroutine to grab another, now that
	// the network transfer has been scheduled

	if pgTrace {
		fmt.Printf("at time %f %s releases source for another packet\n",
			evtmgr.CurrentSeconds(), pcnd.endptName)

		log.Printf("at time %f %s releases source for another packet\n",
			evtmgr.CurrentSeconds(), pcnd.endptName)
	}
	pcnd.cg.cmd <- start
	return 0
}

// From above delivers an ingress rate, for now it is ignored
func (pcnd *pcapNode) fromAbove(parent protocolNode, context any, msg_struct *pgnMsg) (any, int, error) {
	return nil, pcnd.id(), nil
}


