#include "kernel/fcntl.h"
#include "kernel/types.h"
#include "kernel/fs.h"
#include "kernel/stat.h"
#include "user/user.h"

struct vector {
    void** mem;
    int len;
    int cap;
};

void vector_init(struct vector* vec) {
    vec->mem = 0;
    vec->len = 0;
    vec->cap = 0;
}

int vector_realloc(struct vector* vec) {
    int new_cap = 2 * vec->cap + 1;
    void** mem = (void**)malloc(new_cap * sizeof(void*));
    if (!mem) {
        return 1;
    }
    memcpy(mem, vec->mem, vec->len * sizeof(void*));
    if (vec->mem) {
        free(vec->mem);
    }
    vec->mem = mem;
    vec->cap = new_cap;
    return 0;
}

int vector_append(struct vector* vec, void* elem) {
    int ret;
    if (vec->len + 1 > vec->cap) {
        ret = vector_realloc(vec);
        if (ret != 0) {
            return ret;
        }
    }

    vec->mem[vec->len] = elem;
    vec->len += 1;
    return 0;
}

void vector_destroy(struct vector* vec) {
    for (int i = 0; i < vec->len; i++) {
        free(vec->mem[i]);
    }
    if (vec->mem) {
        free(vec->mem);
    }
    vec->mem = 0;
    vec->len = 0;
    vec->cap = 0;
}

struct string {
    char* data;
    int len;
    int cap;
};

int string_init(struct string* str) {
    str->data = (char*)malloc(32);
    if (!str->data) {
        return 1;
    }
    memset(str->data, 0, 32);
    str->len = 0;
    str->cap = 31;
    return 0;
}

void string_clear(struct string* str) {
    str->len = 0;
}

int string_realloc(struct string* str) {
    int new_cap = 2 * str->cap;
    char* data = (char*)malloc(new_cap);
    if (!data) {
        return 1;
    }
    strcpy(data, str->data);
    if (str->data) {
        free(str->data);
    }
    str->data = data;
    str->cap = new_cap;
    return 0;
}

int string_append(struct string* str, const char* s) {
    int len = strlen(s), ret = 0;
    if (str->len + len > str->cap) {
        ret = string_realloc(str);
        if (ret != 0) {
            return ret;
        }
    }

    strcpy(str->data + str->len, s);
    str->len += strlen(s);
    return 0;
}

int current_directory_or_parent_directory(const char* name) {
    if (strlen(name) > 2) {
        return 0;
    }
    if (name[0] == '.') {
        if (strlen(name) > 1) {
            return name[1] == '.';
        } 
        return 1;
    }
    return 0;
}

void usage() {
    fprintf(2, "Usage: find <directory> <pattern>\n");
}

int find(const char* directory, const char* pattern) {
    struct stat st;
    struct dirent subentry;
    int ret, status;
    struct vector vec;
    int fd = open(directory, O_RDONLY);
    // printf("DEBUG: find %s %s\n", directory, pattern);
    if (fd < 0) {
        fprintf(2, "find: fail to open %s\n", directory);
        return 1;
    } 

    if (fstat(fd, &st) != 0) {
        fprintf(2, "find: fail to fstat %s\n", directory);
        close(fd);
        return 1;
    }

    if (st.type != T_DIR) {
        fprintf(2, "find: %s is not a directory\n", directory);
        close(fd);
        return 1;
    }

    status = 0;
    vector_init(&vec);
    for (;;) {
        ret = read(fd, &subentry, sizeof(subentry));
        if (ret < 0) {
            fprintf(2, "find: fail to read directory %s\n", directory);
            status = 1;
            break;
        } else if (ret != sizeof(subentry)) {
            break;
        }
        if (subentry.inum == 0 || current_directory_or_parent_directory(subentry.name)) {
            continue;
        }

        if (strcmp(pattern, subentry.name) == 0) {
            printf("%s/%s\n", directory, subentry.name);
        }
        struct string str;
        string_init(&str);
        string_append(&str, directory);
        string_append(&str, "/");
        string_append(&str, subentry.name);
        // printf("DEBUG: read a subentry %s\n", str.data);
        ret = stat(str.data, &st);
        if (ret != 0) {
            fprintf(2, "find: fail to stat %s\n", str.data);
            status = 1;
        }
        if (st.type == T_DIR) {
            // printf("DEBUG: find a sub directory %s\n", str.data);
            vector_append(&vec, str.data);
        }
    }

    if (status != 0) {
        vector_destroy(&vec);
        return status;
    }
    
    for (int i = 0; i < vec.len; i++) {
        char* name = (char*)(vec.mem[i]);
        ret = find(name, pattern);
        if (ret != 0) {
            fprintf(2, "find: fail to find %s\n", name);
            status = 1;
            break;
        }
    }

    vector_destroy(&vec);
    // printf("DEBUG: FIND EXIT\n");
    return status;
}

int main(int argc, char* argv[]) {
    if (argc < 3) {
        usage();
        exit(1);
    }

    int status = find(argv[1], argv[2]);
    exit(status);
}
