#include "kernel/types.h"           // uint
#include "user/user.h"              // sleep, atoi, fprintf, exit

void usage() {
    fprintf(2, "Usage: sleep <number of ticks>\n");
}

int main(int argc, char* argv[]) {
    if (argc < 2) {
        usage();
        exit(1); 
    }

    int n = atoi(argv[1]);
    int remain = sleep(n);
    if (remain > 0) {
        fprintf(2, "sleep: interrupted by something and sleep for %d ticks\n", n - remain);
        exit(1);
    }
    return 0;
}