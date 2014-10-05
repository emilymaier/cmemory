// Copyright © 2014 Emily Maier

#define _GNU_SOURCE

#include <dlfcn.h>
#include <execinfo.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "_cgo_export.h"

void* (*real_malloc)(size_t);
void* (*real_calloc)(size_t, size_t);
void* (*real_realloc)(void*, size_t);
void (*real_free)(void*);
int inner_initializing = 0;
int instrumenting = 0;
int reentrant = 0;

pthread_once_t initializer = PTHREAD_ONCE_INIT;
pthread_mutex_t mutex;
int initialized = 0;

char start_buf[1024];
char start_buf_pos = 0;

struct block
{
	struct block* next;
};

struct block head;

// Get the real memory allocation functions and set up the mutex.
static void initialize()
{
	inner_initializing = 1;
	real_malloc = (void* (*)(size_t)) dlsym(RTLD_NEXT, "malloc");
	real_calloc = (void* (*)(size_t, size_t)) dlsym(RTLD_NEXT, "calloc");
	real_realloc = (void* (*)(void*, size_t)) dlsym(RTLD_NEXT, "realloc");
	real_free = (void (*)(void*)) dlsym(RTLD_NEXT, "free");
	inner_initializing = 0;

	pthread_mutexattr_t attr;
	pthread_mutexattr_init(&attr);
	pthread_mutexattr_settype(&attr, PTHREAD_MUTEX_RECURSIVE);
	pthread_mutex_init(&mutex, &attr);
	pthread_mutexattr_destroy(&attr);

	head.next = NULL;

	initialized = 1;
}

// Determines whether or not the function that called the allocation function is
// in the Go runtime, as the runtime doesn't expect its "libc" calls to go back
// into Go. For free(), a list of instrumented blocks is used instead.
static int runtime_caller(void* address)
{
	Dl_info info;
	if(&info == NULL)
	{
		return 1;
	}
	if(!dladdr(address, &info))
	{
		printf("dl error: %s\n", dlerror());
		return 1;
	}
	if(info.dli_sname == NULL || !strcmp(info.dli_sname, "x_cgo_thread_start") || !strcmp(info.dli_sname, "_dl_allocate_tls") || !strcmp(info.dli_sname, "pthread_create"))
	{
		return 1;
	}
	return 0;
}

// Gets the C stack trace.
static int get_trace(char*** trace)
{
	void** buf = real_malloc(256 * sizeof(void*));
	int frames = backtrace(buf, 256);
	*trace = backtrace_symbols(buf, frames);
	real_free(buf);
	return frames;
}

// Returns a struct block* corresponding to the argument pointer, or NULL if it
// can't find one.
static struct block* find_block(void* ptr)
{
	struct block* current_block = &head;
	while(current_block->next != NULL)
	{
		if(current_block->next == ptr)
		{
			return current_block;
		}
		current_block = current_block->next;
	}
	return NULL;
}

// Begin instrumenting memory allocation calls.
void start_instrumentation()
{
	pthread_once(&initializer, initialize);
	while(!initialized);
	pthread_mutex_lock(&mutex);
	instrumenting = 1;
	pthread_mutex_unlock(&mutex);
}

void* malloc(size_t size)
{
	pthread_once(&initializer, initialize);
	while(!initialized);
	pthread_mutex_lock(&mutex);
	if(reentrant)
	{
		void* ret = real_malloc(size);
		pthread_mutex_unlock(&mutex);
		return ret;
	}
	reentrant = 1;
	if(!instrumenting || runtime_caller(__builtin_return_address(0)))
	{
		reentrant = 0;
		void* ret = real_malloc(size);
		pthread_mutex_unlock(&mutex);
		return ret;
	}
	void* ptr = real_malloc(size + sizeof(struct block));
	if(ptr == NULL)
	{
		reentrant = 0;
		pthread_mutex_unlock(&mutex);
		return NULL;
	}
	((struct block*) ptr)->next = head.next;
	head.next = ptr;
	ptr = (void*) (((struct block*) ptr) + 1);
	char** trace;
	int frames = get_trace(&trace);
	reentrant = 0;
	pthread_mutex_unlock(&mutex);
	instrumentMalloc(ptr, size, trace, frames);
	pthread_mutex_lock(&mutex);
	reentrant = 1;
	real_free(trace);
	reentrant = 0;
	pthread_mutex_unlock(&mutex);
	return ptr;
}

void* calloc(size_t num, size_t size)
{
	if(inner_initializing)
	{
		void* alloced = &start_buf[start_buf_pos];
		memset(alloced, 0, num * size);
		start_buf_pos += num * size;
		reentrant = 0;
		return alloced;
	}
	pthread_once(&initializer, initialize);
	while(!initialized);
	pthread_mutex_lock(&mutex);
	if(reentrant)
	{
		void* ret = real_calloc(num, size);
		pthread_mutex_unlock(&mutex);
		return ret;
	}
	reentrant = 1;
	if(!instrumenting || runtime_caller(__builtin_return_address(0)))
	{
		reentrant = 0;
		void* ret = real_calloc(num, size);
		pthread_mutex_unlock(&mutex);
		return ret;
	}
	void* ptr = real_calloc(1, num * size + sizeof(struct block));
	if(ptr == NULL)
	{
		reentrant = 0;
		pthread_mutex_unlock(&mutex);
		return NULL;
	}
	((struct block*) ptr)->next = head.next;
	head.next = ptr;
	ptr = (void*) (((struct block*) ptr) + 1);
	char** trace;
	int frames = get_trace(&trace);
	reentrant = 0;
	pthread_mutex_unlock(&mutex);
	instrumentMalloc(ptr, num * size, trace, frames);
	pthread_mutex_lock(&mutex);
	reentrant = 1;
	real_free(trace);
	reentrant = 0;
	pthread_mutex_unlock(&mutex);
	return ptr;
}

void* realloc(void* ptr, size_t size)
{
	pthread_once(&initializer, initialize);
	while(!initialized);
	pthread_mutex_lock(&mutex);
	if(reentrant)
	{
		void* ret = real_realloc(ptr, size);
		pthread_mutex_unlock(&mutex);
		return ret;
	}
	reentrant = 1;
	if(!instrumenting)
	{
		reentrant = 0;
		void* ret = real_realloc(ptr, size);
		pthread_mutex_unlock(&mutex);
		return ret;
	}
	void* real_ptr = (void*) (((struct block*) ptr) - 1);
	struct block* current_block = find_block(real_ptr);
	if(current_block == NULL)
	{
		reentrant = 0;
		void* ret = real_realloc(ptr, size);
		pthread_mutex_unlock(&mutex);
		return ret;
	}
	instrumentFree(ptr);
	real_ptr = real_realloc(real_ptr, size + sizeof(struct block));
	if(real_ptr == NULL)
	{
		reentrant = 0;
		current_block->next = current_block->next->next;
		pthread_mutex_unlock(&mutex);
		return NULL;
	}
	ptr = (void*) (((struct block*) ptr) + 1);
	((struct block*) real_ptr)->next = current_block->next->next;
	current_block->next = real_ptr;
	char** trace;
	int frames = get_trace(&trace);
	reentrant = 0;
	pthread_mutex_unlock(&mutex);
	instrumentMalloc(ptr, size, trace, frames);
	pthread_mutex_lock(&mutex);
	reentrant = 1;
	real_free(trace);
	reentrant = 0;
	pthread_mutex_unlock(&mutex);
	return ptr;
}

static void _free(void* ptr)
{
	real_free(ptr);
	reentrant = 0;
	pthread_mutex_unlock(&mutex);
}

void free(void* ptr)
{
	pthread_once(&initializer, initialize);
	while(!initialized);
	pthread_mutex_lock(&mutex);
	if(reentrant)
	{
		real_free(ptr);
		pthread_mutex_unlock(&mutex);
		return;
	}
	reentrant = 1;
	if(!instrumenting)
	{
		_free(ptr);
		return;
	}
	void* real_ptr = (void*) (((struct block*) ptr) - 1);
	struct block* current_block = find_block(real_ptr);
	if(current_block == NULL)
	{
		_free(ptr);
		return;
	}
	current_block->next = current_block->next->next;
	_free(real_ptr);
	instrumentFree(ptr);
}
