[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Patterned Computation Evaluation Simulator 

# pces
The pces package (Patterned Computation Evaluation Simulator) is used to model applications that execute on a mrnes network model. At the most basic level, an application is compromised of elements (called ‘functions’) that receive messages, execute some computation based on the message and its source, and optionally send messages in response to other functions.  pces model syntax defines the topological structure of these applications (called “Computational Patterns”), describes how messages flow between functions, the function’s individual states, and references methods embedded in the simulator that are called to modify those states as messages are processed.

A foundational principle for pces models is that the simulation time associated with the execution of a function come from a look-up table that holds multiple measurements of that execution time on different CPUs, and under other model dependent parameters (e.g. number of bytes in a message being processed by the function).  Another foundational principle is that pces allow for a large degree of flexibility and specialization in modeling the behavior of modeled software that calls these measured functions.  It does so by organizing the flow of control of a simulation at a level of abstraction and through an API that allows for different ‘class’ dependent behavior of functions.   Control of function behavior with associated virtual time delays explicitly supports representation of the impact of queuing and contention for processor and communication network resources, so that evaluation of pces models can capture the impact of congestion at different levels of the system.

mrnes and pces models are built using the discrete-event simulation paradigm, and use functionality given by the `vrtime, evtq`, and `evtm` packages. The models are explicitly event-oriented (as opposed to “process” oriented models that use discrete-event formulations under the covers, not explicitly viewable by the modeler).   The simulation clock uses 64 bit integers for time ’ticks’, and 64 bit integers for tie- breaking events whose number of clock ticks are precisely the same.   The `vrtime` package allows one to define the time duration of a tick to be whatever time-scale is of interest to the modeler; for our models of computer and communications activities we typically define a tick to represent 1/10 of a nanosecond.


# pces directory

The pces simulator reads various files to prepare for and execute a simulation experiment.  Those files are in .json or .yaml format that are read in and converted to Golang structs at simulator start-up. We describe below the role of methods and data structures found in various files in the MrNesbits package.

#### Files used as part of model-building
The files below have methods that are typically called to either build pces models, or read from file descriptions of models that have been built.
* `desc-cp.go`  This file holds definitions of those structs related to Computational Patterns and their initializations. It contains methods used by the simulator to read in those structs, and also contains methods that a separate external Golang program can use to build and store examples of those structs for specific classes of pces models.
* `desc-crypto.go`  The GUI should present selection options for crypto algorithms that are supported by measurements available to the simulator.  This file holds struct definitions for a file that is read in by the GUI to guide those options, a file that is created by pre-model-building analysis using methods of data structures that holding function timing measurements.
* `desc-map.go`  Prior to model execution, its functions have to be mapped to processors in the declared architecture model. This file holds structs and methods that support creation and access to these mappings.
* `desc-params.go`  pces supports a rich syntax for describing parameter values (e.g. the bandwidth of interfaces) to a model description before a simulation run.  This file contains structs and methods that are used for that purpose.
* `desc-timing.go`  This file holds definition of structs that specify identities of computation functions and their execution timing as a function of the underlaying hardware platform and ‘packet length’ associated with the data being operated on.  The file contains methods for creating these structs from an external Golang program, and methods used by the simulator to read those structs in from file.
#### Files used as part of model-execution
* `class.go`  mrnesbit func belong to ‘classes’ with pre-defined structs and methods used in the simulated execution of those functions.  This file contains the structs, other data structures,  methods, and event handling routines for all of the pre-declared classes.
* `cpf.go`  The pces internals represent its functions through a type it calls a `CmpPtnFuncInst` (Computation Pattern Function Instance).  This file defines this type and methods involved in initializing and simulating the execution of func instances.
* `cpg.go`  pces funcs are organized within so-called ‘Computation Patterns’, instances of which are represented by type `CmpPtnInst`, and which are fundamentally a graph whose nodes are `CmpPtnFuncInsts,` and whose edges describe possible communications between them.   This file contains structs and methods that support construction and traversal through computation patterns.
* `pces.go` Methods in this file are called by the root simulation program to read in the simulation model descriptions, and support interactions with the `mrnes` package.

Copyright 2024 Board of Trustees of the University of Illinois.
See [the license](LICENSE) for details.


