
## Examples Directory
The examples directory holds subdirectories with examples of programs that build and run MrNesbits models.  Each program  declares itself to be ‘main.   The subdirectories are
* buildLib    This subdirectories holds programs that generated dictionaries of MrNesbits model components that can be incorporated into many simulation experiments
* simple-aes-chain   This subdirectory holds programs related to assembling and running a simulation  model where there is just one computation pattern represented, a chain that generates a block of data, encrypts it, decrypts it, and consumes it.
* multi-cmptn   This subdirectory holds programs related to assembling and running a simulation that has the simple-aes-model, but also another model that demonstrates use of the ‘stateful’ computation pattern function.

### BuildLib
The buildLib directory holds programs useful for creating MrNes and MrNesbits models that are then saved in a subdirectory of ‘pre-buillt’ data structures. This makes the assembly of a new model for analysis easier, because modular pieces of that model can be pulled in from the pre-built library.

There are six types of information that are packaged and stored separately.  
* topological descriptions of computational patterns (CP)
* topological description of network devices, and their interconnection to form a communication network for a given experiment
* data structures holding information about initializing the functions in a CP
* tables reporting execution delays for CP functions depending on CPU and packet length
* tables reporting execution delays for switches and routers to transit a message
* tables of network device performance configurations

There are in the buildLib directory example programs that illustrate how these pre-built structures can be created and stored, and how their components can be extracted (modified, and added to) to create the files read in at initialization by the simulation experiment.  

These programs get location information about the files they read and write from the command line.  In practice we put these parameters in a file and read in the file, treating the contexts much as we would if the flags were on the command line.   In the examples here we create two such files, one for programs that solely create files for the pre-built library, one for assembling files for use by the simulator.

The flags on the command line for the ‘pre-built’ programs are
* -prbLib	names a file directory where the pre-built dictionaries go
For all the remaining flags below for file names, the full path to that file is obtained by pre-pending string argument of -prbLib to the file name.
* -cpPrb      names an output file to hold a dictionary of topological description of CPs.  
* -CPInitPrb    names an output file holding a dictionary of records that describe initialization parameters for CP funcs.
* -expPrb    names an output file to hold a dictionary of lists of device configuration parameters that are applied at run-time.  
* -funcExecPrb   names an output file holding a dictionary of tables of timings for various CP func types. 
* -devExecPrb   names an output file holding a dictionary of tables of timings for various network devices types (as a function of device operation, and device manufacturing model)
* -prbCSV    names an input file holding a .csv file that contains timing information, both for functions and for devices
* -topoPrb    names an output file holding a library of topological descriptions of network devices, and their interconnection to form a communication network

In this distribution -cpPrb is set to directory ../pre-built with examples of the pre-built outputs of the model building functions are written.

In principle these files can be in either .json or .yaml format.   

Each of the programs below use only a small number of these arguments, but each is written to accept command lines that have them all, so that we can use just one command line when we execute each.

The programs and their outputs are
* **cp-prb.go**      creates some CP topology descriptions and their CP func initializations, and writes the created dictionaries in the files named by -cpPrb and -CPInitPrb respectively
* **exec-pro.go**    creates a library of tables of function and device timings, placing these in the files named by -funcExecPrb and -devExecPrb respectively
* **exp-prb.go**      creates a library of network performance configuration parameters and writes it to the file named by -expPrb
* **topo-prb.go**     creates a library of communication topologies that can be used in a simulation experiment.

All the programs can be run with outputs placed as configured by running
	% sh runall.sh 
This script uses the command-line configuration file build-pre
### simple-aes-chain
simple-aes-chain holds two programs, one to assemble the model ( **aes-build.go**) and one to run it (**aes-chain.go**).  The model creates a chain of four ‘simple’ functions, mapped onto a network model representing three networks (2 wired, 1 wireless), two switches, four routers, and four hosts.  The chain is mapped across three hosts.

**aes-build.go** reads from the dictionary files created by the programs in buildLib, and uses the same command flags to declare them as input files as the buildLib/args-prb used to declare them as output files.  To describe the creation of build.go’s own output files it includes in file args-build the parameters
* -exptInputLib     the name of a directory into which all of build.go’s subsequent output files are written
As with -cpPrb being the path prefix of the rest of the files, so too is -expInputLib a pathprefix for all of build.go’s output files. These are
* -cpOutput    names an output file where a dictionary of CPs  actually used in the simulation experiment are written
* -cpIntOutput   names an output file where a dictionary of initialization blocks for func used in the the experiment’s CPs is written
* -funcExecOutput   names an output file where a table of CP function timings is written
* -devExecOutput   names an output file where a table of network device timings is written
* -mapOutput   names a file where a table assigning all the CP functions to hosts in the communication network is written
* -expOutput    names a file where a list of network performance parameters is written.

In this distribution  -exptInputLib in simple-aes-chain/args-build is set to subdirectory ../expLib/aes, where the results of running build.go -is args-build are written.

To build the files for the simple-aes experiment run
        % go run aes-build.go -is args-build
with the file args-build containing settings as described above.

The arguments for the program **aes-chain.go** that runs the simple-aes-model are
* -exptLib    path to directory where input files for (potentially) multiple experiments are stored
* -expt   names the subdirectory where the inputs for the specific experiment being run are stored.  The full path to this subdirectory is created using the argument of -exptLib as a path prefix
Full paths to the input files named below are created by using the full path to the experiment’s subdirectory (see above) as a path prefix
* -topoInput   names the file where the communication topology description is stored
* -cpInput  names the file where the computation pattern topology is stored
* -cpInitIinput  names the file where the initialization blocks for functions used in the computation patterns is stored
* -funcExecInput names the file where descriptions of timing related to function executions is stored
* -devExecInput names the file where descriptions of timing related to execution of device operations is stored
* -mapInput names the file where the mapping of computation pattern functions to hosts is stored
* -expInput  names the file where a list of run-time communication network performance configuration parameters is stored
All of the parameters in the list above are required.   Other command line parameters are
* -stop   gives the simulation time (in seconds)  when a simulation run stops. Required parameter.
* -qnetsim  when set uses a ‘quick network simulation’ which computes and uses an end-to-end delay without explicit simulation of the message through communication network devices.  Not a required parameter.
* -rngseed   when included sets the simulation’s random number generation seed using the (positive) integer included as a parameter.   Not a required parameter.

To run the program, 
     % go run aes-chain.go -is args-run

Every time an initiated execution of the chain completes, a message is printed giving the simulation time and the identity of the the computation pattern function where the message stops.

### multi-cmptn
The multi-cmptn directory holds programs **multi-build.go** and **multi.go**  that exactly parallel  the functions in the simple-aes-chain example.   The difference is that the build program creates and the simulation program uses an additional computation pattern (StatefulTest, built by **buildLib/cp-prb.go**) included to demonstrate a computation pattern with ‘stateful’ functions.  The additional pattern has four functions.  One periodically self-initiates a message, but unlike the message in the aes chain, this message carries a payload that represents the ‘valid’ bit of a PKI certificate.  The generator function is stateful, and with some configured periodicity causes the valid bit to be false, while it otherwise would be true.   The message is passed to the next function whose job it is to route the message to one of two alternative destination functions, consumer1 and consumer2.  This function is also stateful.  It may be configured to test the valid bit on the incoming message, or not.  If not the message is always routed to consumer1.   If the function is configured to test the valid bit it will route the message to consumer1 if the bit is valid and to consumer2 if it is not.

Running 
  % go run multi-build.go -is args-build
will create the input arguments read in when 
  % go run multi.go -is args-run
is executed to run.   Like with aes the termination of an initiated transfer is reported by the function where it is detected.    The output of multi should show a mixture of terminations at ‘sink’ (the end function for the aes chain), ‘consumer1’ and ‘consumer2’ , depending on what happens in the StatefulTest computation pattern.


