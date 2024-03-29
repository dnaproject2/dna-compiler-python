#!/usr/bin/env bash
set -ev
oldir=$(pwd)
currentdir=$(dirname $0)
cd $currentdir

../dna_test/example/runall-testing.bash

testdir="../dna_test/example/OffChainOp"
$testdir/runall-testing.bash

for avm in $(ls ${testdir}/*.avm.str)
do
	./dnavm -b $avm
done
cd $oldir
