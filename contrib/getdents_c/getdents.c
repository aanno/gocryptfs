// See ../getdents/getdents.go for some info on why
// this exists.

#include <fcntl.h>
#include <stdio.h>
#include <unistd.h>
#include <stdlib.h>
#include <stdint.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <errno.h>

int main(int argc, char *argv[])
{
    if(argc < 2) {
        printf("Usage: %s PATH\n", argv[0]);
        printf("Run getdents(2) on PATH\n");
        exit(1);
    }

    const char *path = argv[1];
    int fd = open(path, O_RDONLY);
    if (fd == -1) {
        perror("open");
        exit(1);
    }

    char tmp[10000];
    int sum = 0;
    for ( ; ; ) {
        int n = syscall(SYS_getdents64, fd, tmp, sizeof(tmp));
        printf("getdents64 fd%d: n=%d, errno=%d\n", fd, n, errno);
        if (n <= 0) {
            printf("total %d bytes\n", sum);
            break;
        }
        sum += n;
    }
}
