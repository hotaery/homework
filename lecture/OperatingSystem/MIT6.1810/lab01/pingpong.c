#include "kernel/types.h"
#include "user/user.h"

int main(int argc, char* argv[]) {
    int ping[2], pong[2];
    int ret = pipe(ping);
    if (ret != 0) {
        fprintf(2, "pingpong: fail to pipe()\n");
        exit(1); 
    }
    ret = pipe(pong);
    if (ret != 0) {
        fprintf(2, "pingpong: fail to pipe\n");
        exit(1);
    }

    int pid = fork();
    char c;
    if (pid == 0) {
        // child
        close(ping[1]);
        close(pong[0]);
        ret = read(ping[0], &c, 1);
        if (ret < 0) {
            fprintf(2, "pingpong: child process fail to read from ping\n");
            exit(1);
        }
        printf("%d: received ping\n", getpid());
        ret = write(pong[1], "c", 1);
        if (ret < 0) {
            fprintf(2, "pingpong: child process fail to write to pong\n");
            exit(1);
        }
        exit(0);
    } else {
        // parent
        close(ping[0]);
        close(pong[1]);
        ret = write(ping[1], "p", 1);
        if (ret < 0) {
            fprintf(2, "pingpong: parent process fail to write to ping\n");
            exit(1);
        }
        ret = read(pong[0], &c, 1);
        if (ret < 0) {
            fprintf(2, "pingpong: parent process fail to read from pong\n");
            exit(1);
        }
        printf("%d: received pong\n", getpid());
        wait((int*)0);
        exit(0);
    }
}