#include "../../../bpf/bpf.h"

// Convert drum pads to song select.
int song_select(uint8_t *msg) {
	if (msg[0] == 0x89) return BPF_DROP;
	if (msg[0] != 0x99) return BPF_PASS;
	uint8_t song_msg[] = {0xf3, msg[1]};
	midi_write(song_msg, 2);
	return BPF_DROP;
}
