
StatefulFlag Computation Pattern

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
| branch     | (src, marked)   | testFlag    | branch    | unused |


Parameters: Response

| Func label | inEdge           | OutEdge             | Period | Choice    |
|------------|------------------|---------------------|--------|-----------|
| src        | (src, initiate)  | (branch,marked)     | >0     | unused    |
| branch     | (src, marked)    | (consumer1, result) | 0      | consumer1 |
| branch     | (src, marked)    | (consumer2, result) | 0      | consumer2 |
| consumer1  | (branch, result) | empty               | 0      | N/A       |
| consumer2  | (branch, result) | empty               | 0      | N/A       |

