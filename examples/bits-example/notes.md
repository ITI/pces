
## StatefulFlag Computation Pattern

Nodes
| Func variable | Func type | Func label | Execution Type |
|---------------|-----------|------------|----------------|
| src           | mark      | src        | stateful       |
| branch        | testFlag  | branch     | stateful       |
| consumer1     | process   | consumer1  | static         |
| consumer2     | process   | consumer2  | static         |

Edges
| Graph Edge          | msg type on edge | annotation on edge |
|---------------------|------------------|--------------------|
| (src, src)          | initiate         | initiate           |
| (src, branch)       | marked           | marked             |
| (branch, consumer1) | result           | consumer1          |
| (branch, consumer2) | result           | consumer2          |

Message Types
| msg type | pckt len | msg len |
|----------|----------|---------|
| initiate | 250      | 1500    |
| marked   | 250      | 1500    |
| result   | 250      | 1500    |

Parameters: Action Selection (Stateful)
| Func label | InEdge          | Action code | Func Type | Limit  |
|------------|-----------------|-------------|-----------|--------|
| src        | (src, initiate) | mark        | mark      | unused |
| branch     | (src, marked)   | testCert    | branch    | unused |

State map attributes
| Func label | state[ ]   | use                                                        |
|------------|------------|------------------------------------------------------------|
| src        | failperiod | periodicity of failing cert bit                            |
| branch     | testflag   | bool indicating whether to have cert flag impact branching |

Parameters: Response
| Func label | inEdge           | OutEdge             | Period | Choice    |
|------------|------------------|---------------------|--------|-----------|
| src        | (src, initiate)  | (branch,marked)     | >0     | unused    |
| branch     | (src, marked)    | (consumer1, result) | 0      | consumer1 |
| branch     | (src, marked)    | (consumer2, result) | 0      | consumer2 |
| consumer1  | (branch, result) | empty               | 0      | N/A       |
| consumer2  | (branch, result) | empty               | 0      | N/A       |

## cryptoPerf Computation Pattern

Nodes
| Func variable         | Func type | Func label | Execution Type |
|-----------------------|-----------|------------|----------------|
| src                   | generate  | src        | stateful       |
| crypto                | select    | crypto     | stateful       |
| host-x (x=0,1, … n-1) | process   | host-x     | static         |

Edges
| Graph Edge       | msg type on edge | annotation on edge |
|------------------|------------------|--------------------|
| (src, src)       | initiate         | initiate           |
| (src, crypto)    | data             | crypto             |
| (crypto, src)    | data             | src                |
| (crypto, host-x) | data             | host-x             |
| (host-x, crypto) | data             | crypto             |

Message Types
| msg type | pckt len | msg len |
|----------|----------|---------|
| initiate | 1000     | 1500    |
| data     | 1000     | 1500    |

Parameters: Action Selection (Stateful)
| Func label         | InEdge           | Action code | Func Type   | Limit  |
|--------------------|------------------|-------------|-------------|--------|
| src                | (src, initiate)  | cycle       | generate    | unused |
| src                | (crypto, data)   | passThru    | passThru    | unused |
| crypto             | (src, marked)    | select      | decrypt-aes | unused |
| crypto             | (host-x, data)   | passThru    | encrypt-aes | unused |
| host-x (x=0,..n-1) | (crypto, host-x) | passThru    |             | unused |

State map attributes
| Func label | state[ ] | use                                                        |
|------------|----------|------------------------------------------------------------|
| src        | hosts    | number of hosts to select from                             |
| branch     | testflag | bool indicating whether to have cert flag impact branching |

Parameters: Response

| Func label | inEdge           | OutEdge             | Period | Choice    |
|------------|------------------|---------------------|--------|-----------|
| src        | (src, initiate)  | (branch,marked)     | >0     | unused    |
| branch     | (src, marked)    | (consumer1, result) | 0      | consumer1 |
| crypto     | (src, marked)    | (consumer2, result) | 0      | consumer2 |
| consumer1  | (branch, result) | empty               | 0      | N/A       |
| consumer2  | (branch, result) | empty               | 0      | N/A       |

