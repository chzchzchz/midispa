package bpf

import (
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

struct ubpf_vm* load_bpf(const char* s)
{
	struct ubpf_vm* vm;
	int rc;
	vm = ubpf_create();
	assert(vm);

	off_t len = 0;
	void* elf = open_elf(s, &len);
	assert(elf);

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
	uint64_t ret_val;
	int rc = ubpf_exec(vm, mem, len, &ret_val);
	assert (rc == 0);
	return (int)ret_val;
}

typedef struct ubpf_vm ubpf_vm;
*/
import "C"

const (
	DROP = iota
	PASS
	DONE
)

type BPF struct {
	path string
	vm   *C.ubpf_vm
}

func NewBPF(p string) *BPF {
	cs := C.CString(p)
	vm := C.load_bpf(cs)
	C.free(unsafe.Pointer(cs))
	if vm == nil {
		return nil
	}
	ret := &BPF{p, vm}
	runtime.AddCleanup(ret, func(vm *C.ubpf_vm) { C.ubpf_destroy(vm) }, vm)
	return ret
}

func (bpf *BPF) Run(dat []byte) int {
	v := C.run_bpf(bpf.vm, unsafe.Pointer(&dat), C.int(len(dat)))
	return int(v)
}
