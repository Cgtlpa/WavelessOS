#include "service.h"
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/wait.h>
#include <string.h>
#include <dirent.h>

static int run_script(const char *name, const char *action)
{
	char path[256];
	snprintf(path, sizeof(path), "/etc/init.d/%s", name);

	if (access(path, X_OK) != 0)
		return -1;

	pid_t pid = fork();
	if (pid == 0) {
		execl(path, path, action, NULL);
		_exit(1);
	}

	int status;
	waitpid(pid, &status, 0);
	return WIFEXITED(status) ? WEXITSTATUS(status) : -1;
}

int svc_start(const char *name)
{
	return run_script(name, "start");
}

int svc_stop(const char *name)
{
	return run_script(name, "stop");
}

int svc_restart(const char *name)
{
	return run_script(name, "restart");
}

int svc_status(const char *name)
{
	return run_script(name, "status");
}

int svc_list(void)
{
	DIR *d = opendir("/etc/init.d");
	if (!d)
		return -1;

	struct dirent *e;
	while ((e = readdir(d))) {
		if (e->d_name[0] == '.')
			continue;
		printf("  %s\n", e->d_name);
	}
	closedir(d);
	return 0;
}
