#!/bin/bash

set -eoux pipefail

mkdir -p output

killall midiclock midirec midifilter midiloop || true

# pads into song select slots
../midifilter/midifilter -i "W-BW61 MIDI 1" -name f1 -bpf ../midifilter/examples/song_select.elf &
pids="$!"

# play button into play/stop toggle button
../midifilter/midifilter -i "W-BW61 MIDI 1" -name f2 -bpf ../midifilter/examples/mmc_play_stop.elf &
pids="$! $pids"
sleep 1s

# clock start/stop is controlled by play button
../midiclock/midiclock -i f2 -bpm 120 &
pids="$! $pids"

sleep 1s

# use song select pads to select playing loops
../midiloop/midiloop -i f1 -c midiclock -dir output/slots -o "MIDI4x4 MIDI Out 1" &

# use song select pads to select save slot
./midirec -q -i f1 -m midiclock -o output/ &
pids="$! $pids"

# connect clock to external midi drum machine
clock_port=`aconnect -l | grep midiclock | cut -f1 -d: | awk ' {print $2 } ' | head -n1`
midi_port=`aconnect -l | grep MIDI4x4 | cut -f1 -d: | awk ' { print $2 } ' | head -n1`
aconnect $clock_port:0 $midi_port:2

wait $pids