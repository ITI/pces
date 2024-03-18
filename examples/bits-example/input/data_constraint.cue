package main

// example data constraint file for the files in /input and docs/yaml
#ExecType: "static" | "deterministic" | "random" |"stateful"
#apicpname: "client-server" | "simple-AES-chain" | "simple-det-test" | "simple-random-branch" | "simple-RSA-chain" | "simple-bit-test" | "encryptPerf" 
#apicptypes: "RSAChain" | "SimpleWeb" | "AESChain" | "DeterministicTest" | "RandomBranch" | "StatefulTest" | "encryptPerf"
#apihostnames: "hostNetA1" | "hostNetB1" | "hostNetA2" | "hostNetT1" | "CA" | "host-3" | "host-2" | "host-1" | "host-0" | "src" | "ssl"

#apifunctypes: {
    "client-server": "generate" | "serve"
    "simple-AES-chain": "generate" | "AES-encrypt" | "AES-decrypt" | "consume"
    "simple-det-test": "mark" | "testFlag" | "process"
    "simple-random-branch": "generate" | "branch" | "consume"
    "encryptPerf": "process" | "select" | "generate"
}

#apifunclabels: {
    "client-server": "client" | "server" | ""
    "simple-AES-chain": "src" | "encrypt" | "decrypt" | "consumer" | ""
    "simple-det-test": "src" | "select" | "consumer1" | "consumer2" | ""
    "simple-random-branch": "src1" | "src2" | "select" | "consumer1" | "consumer2" | ""
    "simple-RSA-chain": "src" | "decrypt" | "encrypt" | "sink" | "branch" | ""
    "simple-bit-test": "branch" | "consumer1" | "consumer2" | "src" | ""
    "encryptPerf": "host-0" | "host-1" | "host-2" | "host-3" | "crypto" | "src" | "data" | "initiate"
}

#apimsgtypes: {
    "client-server": "initiate" | "request" | "response"  | ""
    "simple-AES-chain": "initiate" | "data" | "encrypted" | "decrypted"  | "" | "consume"
    "simple-det-test": "initiate" | "marked" | "result"  | ""
    "simple-random-branch": "initiate" | "data" | "selected"  | ""
	"simple-RSA-chain": "encrypted" | "decrypted" | "data" | "initiate" | ""
	"simple-bit-test": "result" | "marked" | "" | "initiate"
    "encryptPerf": "data" | "initiate"
}

// should these be part of the data files?
#paramObject: "Switch" | "Router" | "Host" | "Filter" | "Interface" | "Network"
#Attrbname: "media" | "*" | "name" | "group" | "CPU" | "model" | "scale"
//#Attrbvalue: "wired" | "wireless" | "" | "crypto"
#paramOptions: "CPU" | "model" | "accelerator" | "bandwidth" | "buffer" | "capacity" | "latency" | "delay" | "MTU" | "trace"

// data files?
#apicpu: "x86" | "pentium" | "M2" | "M1" | "accel-x86" | ""
#apifunctypesFullList: "generate" | "serve" | "AES-encrypt" | "AES-decrypt" | "consume" | "mark" | "testFlag" | "process" | "branch" | "RSA-decrypt" | "RSA-encrypt" | "pki-cache" | "pki-server" | "decrypt-aes" | "encrypt-aes" | "passThru" | "encrypt-rsa" | "decrypt-rsa" | "rtt"


// data files? this only validates topo.yaml in /input and topo.yaml.txt under docs/yaml
#apinetnames: "srcNet" | "dstNet" | "transitNet" | "private" | "public"
#apimediatypes: "" | "wireless" | "wired"
#apirouternames: "Rtr:(srcNet-transitNet).[1]" | "Rtr:(dstNet-transitNet).[1]" | "" | "pvtHub" | "pvtRtr"
#apifilternames: "ssl" | ""
#devtypes: "Host" | "Router" | "Switch" | "Filter"
#apicpunames: ""
#apimodelnames: "Dell" | "cisco" | "" | "fortinet" | "Apple"
#apiswitchnames: "srcSwitch"
#apiintrfcnames: "intrfc@Rtr:(srcNet-transitNet).[1][.1]" | "intrfc@Rtr:(srcNet-transitNet).[1][.2]" | "intrfc@Rtr:(dstNet-transitNet).[1][.3]" | "intrfc@Rtr:(dstNet-transitNet).[1][.4]" | "intrfc@dstHub[.6]" | "intrfc@hostNetA1[.7]" | "intrfc@pvtHub[.7]" | "intrfc@pvtHub[.4]" | "intrfc@pvtRtr[.12]" | "intrfc@pvtRtr[.5]" | "intrfc@pvtRtr[.3]" | "intrfc@CA[.11]" | "intrfc@host-3[.10]" | "intrfc@host-2[.9]" | "intrfc@host-1[.8]" | "intrfc@host-0[.6]" | "intrfc@src[.0]" | "intrfc@ssl[.2]" | "intrfc@ssl[.1]"
#apidevnames: "Rtr:(srcNet-transitNet).[1]" | "pvtHub" | "pvtRtr" | "CA" | "host-3" | "host-2" | "host-1" | "host-0" | "src" | "ssl"
#apibrdnames: ""
#apifiltertype: "Filter"
//#apicpu: "x86" | "pentium" | "M2" | "M1" | "accel-x86" | ""
//#apihostnames: "hostNetA1" | "hostNetB1" | "hostNetA2" | "hostNetT1" | "host-0" |"host-1" | "host-2" | "host-3" | "src" | "ssl" | "CA"
