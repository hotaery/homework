#include "kernel/types.h"
#include "user/user.h"

void prime(int input) {
    int elimitates, num, ret;
    int channel[2];
    int forked = 0, status = 0;
    ret = read(input, &elimitates, sizeof(elimitates));
    if (ret <= 0) {
        fprintf(2, "primes: %d fail to read from pipe\n", getpid());
        exit(1);
    }

    // printf("DEBUG: process[%d] with elimitator[%d] has started\n", getpid(), elimitates);
    
    for (;;) {
        ret = read(input, &num, sizeof(num));
        if (ret < 0) {
            fprintf(2, "primes: %d fail to read from pipe\n", getpid());
            status = 1;
            break;
        } else if (ret == 0) {
            printf("prime %d\n", elimitates);
            status = 0;
            break;
        } else if (num % elimitates != 0) {
            if (forked) {
                ret = write(channel[1], &num, sizeof(num));
                if (ret != sizeof(num)) {
                    fprintf(2, "primes: %d fail to write %d to child\n", getpid(), num);
                    status = 1;
                    break;
                }
            } else {
                ret = pipe(channel);
                if (ret != 0) {
                    fprintf(2, "primes: %d fail to call pipe()\n", getpid());
                    status = 1;
                    break;
                }
                ret = fork();
                if (ret == 0) {
                    // child
                    close(channel[1]);
                    prime(channel[0]);
                } else if (ret > 0) {
                    // parent
                    close(channel[0]);
                    forked = 1;
                    ret = write(channel[1], &num, sizeof(num));
                    if (ret != sizeof(num)) {
                        fprintf(2, "primes: %d fail to write %d to child\n", getpid(), num);
                        status = 1;
                        break;
                    } 
                } else {
                    fprintf(2, "primes: %d fail to fork\n", getpid());
                    status = 1;
                    break;
                }
            }
        }
    }

    if (forked) {
        close(channel[1]);
        wait((int*)0);
    }
    close(input);
    exit(status);
}

int main(int argc, char* argv[]) {
    int ret;
    int channel[2];
    ret = pipe(channel);
    if (ret != 0) {
        fprintf(2, "primes: fail to call pipe\n");
        exit(1);
    }
    ret = fork();
    if (ret == 0) {
        // child
        close(channel[1]);
        prime(channel[0]);
    } else if (ret > 0) {
        // parent
        int status = 0;
        close(channel[0]);
        for (int i = 2; i <= 35; i++) {
            ret = write(channel[1], &i, sizeof(i));
            if (ret != sizeof(i)) {
                fprintf(2, "primes: fail to write %d to pipe\n", i);
                status = 1;
                break;
            }
        }
        close(channel[1]);
        wait((int*)0);
        exit(status);
    } else {
        fprintf(2, "primes: fail to fork()\n");
        exit(1);
    }
    return 0;
}
