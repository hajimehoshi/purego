// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !cgo && (amd64 || arm64)

package fakecgo

import "unsafe"

//go:nosplit
func _cgo_sys_thread_start(ts *ThreadStart) {
	var attr pthread_attr_t
	var ign, oset sigset_t
	var p pthread_t
	var size size_t
	var err int

	// fprintf(stderr, "runtime/cgo: _cgo_sys_thread_start: fn=%p, g=%p\n", ts->fn, ts->g); // debug
	sigfillset(&ign)
	pthread_sigmask(SIG_SETMASK, &ign, &oset)

	pthread_attr_init(&attr)
	pthread_attr_getstacksize(&attr, &size)
	// Leave stacklo=0 and set stackhi=size; mstart will do the rest.
	ts.g.stackhi = uintptr(size)

	err = _cgo_try_pthread_create(&p, &attr, unsafe.Pointer(threadentry_trampolineABI0), ts)

	pthread_sigmask(SIG_SETMASK, &oset, nil)

	if err != 0 {
		print("fakecgo: pthread_create failed: ")
		println(err)
		abort()
	}
}

// threadentry_trampolineABI0 maps the C ABI to Go ABI then calls the Go function
//
//go:linkname x_threadentry_trampoline threadentry_trampoline
var x_threadentry_trampoline byte
var threadentry_trampolineABI0 = &x_threadentry_trampoline

//go:nosplit
func threadentry(v unsafe.Pointer) unsafe.Pointer {
	var ss stack_t
	ts := *(*ThreadStart)(v)
	free(v)

	// On NetBSD, a new thread inherits the signal stack of the
	// creating thread. That confuses minit, so we remove that
	// signal stack here before calling the regular mstart. It's
	// a bit baroque to remove a signal stack here only to add one
	// in minit, but it's a simple change that keeps NetBSD
	// working like other OS's. At this point all signals are
	// blocked, so there is no race.
	ss.ss_flags = SS_DISABLE
	sigaltstack(&ss, nil)

	setg_trampoline(setg_func, uintptr(unsafe.Pointer(ts.g)))

	// faking funcs in go is a bit a... involved - but the following works :)
	fn := uintptr(unsafe.Pointer(&ts.fn))
	(*(*func())(unsafe.Pointer(&fn)))()

	return nil
}

// here we will store a pointer to the provided setg func
var setg_func uintptr

//go:nosplit
func x_cgo_init(g *G, setg uintptr) {
	var size size_t
	var attr *pthread_attr_t

	/* The memory sanitizer distributed with versions of clang
	   before 3.8 has a bug: if you call mmap before malloc, mmap
	   may return an address that is later overwritten by the msan
	   library.  Avoid this problem by forcing a call to malloc
	   here, before we ever call malloc.

	   This is only required for the memory sanitizer, so it's
	   unfortunate that we always run it.  It should be possible
	   to remove this when we no longer care about versions of
	   clang before 3.8.  The test for this is
	   misc/cgo/testsanitizers.

	   GCC works hard to eliminate a seemingly unnecessary call to
	   malloc, so we actually use the memory we allocate.  */

	setg_func = setg
	attr = (*pthread_attr_t)(malloc(unsafe.Sizeof(*attr)))
	if attr == nil {
		println("fakecgo: malloc failed")
		abort()
	}
	pthread_attr_init(attr)
	pthread_attr_getstacksize(attr, &size)
	// runtime/cgo uses __builtin_frame_address(0) instead of `uintptr(unsafe.Pointer(&size))`
	// but this should be OK since we are taking the address of the first variable in this function.
	g.stacklo = uintptr(unsafe.Pointer(&size)) - uintptr(size) + 4096
	pthread_attr_destroy(attr)
	free(unsafe.Pointer(attr))
}
