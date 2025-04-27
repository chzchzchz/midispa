#ifndef MIDISPA_BPF_H
#define MIDISPA_BPF_H

#include <stddef.h>
#include <stdint.h>

#define BPF_DROP	0
#define BPF_PASS	1
#define BPF_DONE	2

static void (* const midi_write)(const uint8_t*, size_t) = (void *) 1;

#endif
