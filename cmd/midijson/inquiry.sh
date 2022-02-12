#!/bin/bash
set -eou pipefail

dev="MIDI4x4%20MIDI%202"
curl --header "Accept: application/json; content=\"sysex/DeviceInquiryResponse\"" \
	"http://localhost:4567/$dev/sysex/DeviceInquiryRequest" -XPOST -d'{"DeviceId": 0}'

