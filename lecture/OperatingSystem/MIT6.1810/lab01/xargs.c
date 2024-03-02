#include "kernel/types.h"
#include "kernel/param.h"
#include "user/user.h"

struct fd_with_buffer {
    char* buffer;
    int offset;
    int end;
    int cap;
    int eof;
    int err;
    int fd;
};

int read_char(struct fd_with_buffer* fd, char* c) {
    if (fd->offset < fd->end) {
        *c = fd->buffer[fd->offset++];
        return 1;
    }
    if (fd->err || fd->eof) {
        return fd->eof ? 0 : -1;
    }

    int ret = read(fd->fd, fd->buffer, fd->cap);
    if (ret < 0) {
        fprintf(2, "xargs: fail to read %d\n", fd->fd);
        fd->err = 1;
        return -1;
    } else if (ret == 0) {
        fd->eof = 1;
        return 0;
    } else {
        fd->offset = 0;
        fd->end = ret;
        return read_char(fd, c);
    }
}

int read_arg_list(struct fd_with_buffer* fd, char* argv[]) {
    char c;
    int ret;
    char* arg = (char*)malloc(64);
    char* curr = arg;
    int i = 0;
    while ((ret = read_char(fd, &c)) == 1) {
        // printf("DEBUG: go into read_char %c, %d, %d\n", c, c, ' ');
        if (c == '\n') {
            break;
        }
        if (c == ' ') {
            if (curr > arg) {
                char* arg_tmp = (char*)malloc(curr - arg + 1);
                *curr = '\0';
                strcpy(arg_tmp, arg);
                arg_tmp[curr - arg] = '\0';
                // printf("DEBUG get an arg %s\n", arg_tmp);
                argv[i++] = arg_tmp;
                curr = arg;
            }
        } else {
            *curr = c;
            curr++;
        }
    }
    if (ret < 0) {
        return ret;
    }
    if (curr > arg) {
        char* arg_tmp = (char*)malloc(curr - arg + 1);
        *curr = '\0';
        strcpy(arg_tmp, arg);
        arg_tmp[curr - arg] = '\0';
        // printf("DEBUG get an arg %s\n", arg_tmp);
        argv[i++] = arg_tmp;
        curr = arg;
    }
    // printf("DEBUG: go into read_arg_list %d\n", i);
    if (i == 0) {
        return 0;
    }
    return i;
}

int xargs(char* argv[], int arg_num) {
    struct fd_with_buffer fd;
    fd.fd = 0;
    fd.buffer = (char*)malloc(256);
    fd.cap = 256;
    fd.end = 0;
    fd.offset = 0;
    fd.eof = 0;
    fd.err = 0;

    char* child_argv[MAXARG+1];
    // printf("DEBUG: %s %d\n", argv[0], arg_num);
    for (int i = 0; i < arg_num; i++) {
        // printf("DEBUG: %s\n", argv[i]);
        child_argv[i] = argv[i];
    }

    int cnt = 0;
    while ((cnt = read_arg_list(&fd, child_argv + arg_num)) > 0) {
        child_argv[arg_num + cnt] = 0;
        int ret = fork();
        if (ret == 0) {
            // child
            exec(argv[0], child_argv);
            exit(1);
        } else if (ret > 0) {
            // parent
            wait((int*)0);
            for (int i = 0; i < arg_num + cnt; i++) {
                // printf("DEBUG: free %d, %s\n", i, child_argv[i]);
                if (i >= arg_num) {
                    free(child_argv[i]);
                }
            } 
        } else {
            fprintf(2, "xargs: fail to fork\n");
            return 1;
        }
    }
    free(fd.buffer);
    return fd.err;
}


int main(int argc, char* argv[]) {
    return xargs(argv+1, argc-1);
}
