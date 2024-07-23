#!/bin/bash

x=`bc -l <<< "$1 * 64"`
lsb=`python -c "print(int($x) & 0x7f)"`
msb=`python -c "print(int($x) >> 7)"`
echo setting bpm $1 $msb $lsb
midisend -p midiclock -s "B0 10 "`printf "%02x" $msb`
midisend -p midiclock -s "B0 30 "`printf "%02x" $lsb`