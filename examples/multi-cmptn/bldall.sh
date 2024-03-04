#!/bin/sh
echo "Running cp-prb"
go run cp-prb.go -is args-prb
echo "Running topo-prb"
go run topo-prb.go -is args-prb
echo "Running exec-prb"
go run exec-prb.go -is args-prb
echo "Running exc-prb"
go run exp-prb.go -is args-prb
echo "Running multi-build"
go run multi.go -is args-build
echo "Done!"
