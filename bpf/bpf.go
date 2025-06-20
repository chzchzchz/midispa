//go:build bpf

package bpf

import (
	"io"
	"runtime"
	"unsafe"
)

/*
#cgo LDFLAGS: -lubpf

#include <stdio.h>
#include <ubpf.h>
#include <assert.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <inttypes.h>
#include <stdlib.h>
#include <string.h>

static void* open_elf(const char* path, off_t* len) {
	struct stat s;
	if (stat(path, &s) != 0) return NULL;
	*len = s.st_size;

	int fd = open(path, O_RDONLY);
	if (fd == -1) return NULL;

	void* ret = mmap(NULL, *len, PROT_READ, MAP_PRIVATE, fd, 0);
	close(fd);

	return ret != MAP_FAILED ? ret : NULL;
}

void midi_write(uint8_t* data, size_t bytes, uint64_t, uint64_t, uint64_t, uint8_t* mem) {
	uint8_t* end = &mem[-2];
	uint8_t* pkt = &mem[*end - 2];
	pkt[0] = bytes;
	memcpy(pkt + 1, data, bytes);
	*end += bytes + 1;
}


static uint8_t* _global_data;
static uint64_t _global_data_size;

uint64_t
do_data_relocation(
    void* user_context,
    const uint8_t* map_data,
    uint64_t map_data_size,
    const char* symbol_name,
    uint64_t symbol_offset,
    uint64_t symbol_size)
{
	(void)user_context; // unused
	(void)symbol_name;  // unused
	(void)symbol_size;  // unused
	if (!_global_data) {
		_global_data = calloc(map_data_size, sizeof(uint8_t));
		_global_data_size = map_data_size;
		memcpy(_global_data, map_data, map_data_size);
	}

	const uint64_t* target_address = (const uint64_t*)((uint64_t)_global_data + symbol_offset);
	return (uint64_t)target_address;
}

// use the one from test.c if it matters
bool
data_relocation_bounds_check(void*, uint64_t, uint64_t) { return true; }

struct ubpf_vm* load_bpf(const char* s)
{
	struct ubpf_vm* vm;
	int rc;
	vm = ubpf_create();
	assert(vm);

	// idx 0 will optimize as undefined behavior with -O2, so begin with idx 1
	ubpf_register(vm, 1, "midi_write", as_external_function_t(midi_write));
	ubpf_register_data_relocation(vm, NULL, do_data_relocation);
	ubpf_register_data_bounds_check(vm, NULL, data_relocation_bounds_check);

	off_t len = 0;
	void* elf = open_elf(s, &len);
	assert(elf);

	_global_data = NULL;
	_global_data_size = 0;

	char* errmsg;
	rc = ubpf_load_elf(vm, elf, len, &errmsg);
	munmap(elf, len);
	if (rc != 0) {
		fprintf(stderr, "load_bpf: %s\n", errmsg);
		return NULL;
	}
	return vm;
}

int run_bpf(struct ubpf_vm* vm, void* mem, int len) {
	uint64_t ret;
	uint8_t *mem8 = mem;
	int rc = ubpf_exec(vm, mem8 + 2, len - 2, &ret);
	assert (rc == 0);
	return (int)ret;
}

typedef struct ubpf_vm ubpf_vm;
*/
import "C"

const (
	DROP = iota
	PASS
	DONE
)

const MaxMessageBytes = 64
const BufBytes = 256

type BPF struct {
	path string
	vm   *C.ubpf_vm
	w    io.Writer
	buf  []byte
}

func NewBPF(p string, w io.Writer) *BPF {
	cs := C.CString(p)
	vm := C.load_bpf(cs)
	C.free(unsafe.Pointer(cs))
	if vm == nil {
		return nil
	}
	ret := &BPF{p, vm, w, make([]byte, BufBytes)}
	runtime.AddCleanup(ret, func(vm *C.ubpf_vm) { C.ubpf_destroy(vm) }, vm)
	return ret
}

func (bpf *BPF) Run(dat []byte) int {
	if len(dat) >= MaxMessageBytes {
		return PASS
	}

	// Adding [0] to the pointer should give the next place to write.
	bpf.buf[0] = byte(len(dat)) + 2
	// The first byte is used for the message length.
	bpf.buf[1] = byte(len(dat))
	// The message follows the message length byte.
	copy(bpf.buf[2:], dat)

	v := C.run_bpf(bpf.vm, unsafe.Pointer(&bpf.buf[0]), C.int(BufBytes))

	// Set mutated message and skip over it.
	copy(dat, bpf.buf[2:len(dat)+2])
	idx := 2 + bpf.buf[1]
	remaining := int(bpf.buf[0] - idx)

	// Write enqueued data, if any.
	for remaining > 0 {
		l := bpf.buf[idx]
		msg := bpf.buf[idx+1 : idx+1+l]
		if _, err := bpf.w.Write(msg); err != nil {
			panic(err)
		}
		remaining -= int(1 + l)
		idx += 1 + bpf.buf[idx]
	}

	return int(v)
}
