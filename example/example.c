#include <stdlib.h>

char* reachable_block;
char* leaked_block;
char* freed_block;

void leak()
{
	int i;
	for(i = 0; i < 1024; i++)
	{
		leaked_block = malloc(32);
	}
	leaked_block = NULL;
}

void c_main()
{
	reachable_block = malloc(16);
	leak();
	freed_block = malloc(64);
	free(freed_block);
	leaked_block = (char*) 1; // prevent call stack optimization
}
