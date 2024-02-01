#!/bin/sh
echo "calling cp-prb.go"
go run cp-prb.go -is args-prb
echo "calling exec-prb.go"
go run exec-prb.go -is args-prb
echo "calling exp-prb.go"
go run topo-prb.go -is args-prb
echo "Done!"
