#include "../../../bpf/bpf.h"
#include <stdbool.h>
#include <string.h>

bool do_play = true;

// Toggle MMC play between play and stop.
int mmc_play_to_start(uint8_t *msg) {
	if (msg[0] != 0xf0) {
		return BPF_DROP;
	}
	const uint8_t mmc_stop[] = {
		0xF0, 0x7F, 0x7F, 0x06, 0x01, 0xF7
	};
	const uint8_t mmc_play[] = {
		0xF0, 0x7F, 0x7F, 0x06, 0x02, 0xF7
	};
	// Only accept incoming play messages.
	if (memcmp(msg, mmc_play, sizeof(mmc_play)) != 0) {
		return BPF_DROP;
	}
	if (do_play) {
		do_play = false;
		return BPF_PASS;
	}
	do_play = true;
	midi_write(mmc_stop, sizeof(mmc_stop));
	return BPF_DROP;
}
