#include "../../../bpf/bpf.h"
#include <stdint.h>

int no_clock(uint8_t *msg) { return msg[0] == 0xf8 ? BPF_DROP : BPF_PASS; }