#include "../../../bpf/bpf.h"

// Command bytes of the two channel messages this filter reads and writes,
// each masked to its high nibble.
#define NOTE_ON_CMD 0x90
#define CC_CMD      0xb0

// The Volca Drum's gain control change (cc/model.go DryGain), the parameter
// a drum NoteOn's velocity is mapped onto.
#define GAIN_CC 52

// The highest raw MIDI channel (0-based) the drum map covers, being channels
// 1 through 6.
#define DRUM_CHANNEL_LAST 5

// Map each drum NoteOn's velocity onto the Volca Drum's gain CC on the same
// channel, then let the NoteOn through. Anything else, including a NoteOn on
// another channel, passes through untouched.
int volca_drum_velocity(uint8_t *msg) {
	uint8_t status = msg[0];
	if ((status & 0xf0) != NOTE_ON_CMD) return BPF_PASS;
	uint8_t channel = status & 0x0f;
	if (channel > DRUM_CHANNEL_LAST) return BPF_PASS;
	uint8_t gain_msg[] = {CC_CMD | channel, GAIN_CC, msg[2]};
	midi_write(gain_msg, sizeof(gain_msg));
	return BPF_PASS;
}
