#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <sys/reboot.h>
#include <sys/sysmacros.h>
#include <signal.h>
#include <fcntl.h>
#include <string.h>
#include <errno.h>

static int running = 1;

static void try_mount(const char *src, const char *target, const char *type, unsigned long flags, const char *opts)
{
	int r = mount(src, target, type, flags, opts);
	if (r != 0) {
		int e = errno;
		fprintf(stderr, "init: mount %s on %s failed: %s\n", src, target, strerror(e));
	}
}

static void mount_fs(void)
{
	try_mount("proc", "/proc", "proc", 0, NULL);
	try_mount("sysfs", "/sys", "sysfs", 0, NULL);
	try_mount("tmpfs", "/run", "tmpfs", 0, "mode=0755");
	try_mount("devtmpfs", "/dev", "devtmpfs", 0, NULL);

	if (mkdir("/dev/pts", 0755) == 0 || errno == EEXIST)
		try_mount("devpts", "/dev/pts", "devpts", 0, NULL);

	if (access("/dev/console", F_OK) != 0)
		mknod("/dev/console", S_IFCHR | 0600, makedev(5, 1));
	if (access("/dev/null", F_OK) != 0)
		mknod("/dev/null", S_IFCHR | 0666, makedev(1, 3));
	if (access("/dev/zero", F_OK) != 0)
		mknod("/dev/zero", S_IFCHR | 0666, makedev(1, 5));
	if (access("/dev/tty", F_OK) != 0)
		mknod("/dev/tty", S_IFCHR | 0666, makedev(5, 0));
}

static void sighandler(int sig)
{
	switch (sig) {
	case SIGCHLD:
		while (waitpid(-1, NULL, WNOHANG) > 0);
		break;
	case SIGTERM:
	case SIGINT:
		running = 0;
		break;
	case SIGQUIT:
		reboot(RB_AUTOBOOT);
		break;
	}
}

static void setup_signals(void)
{
	struct sigaction sa;
	memset(&sa, 0, sizeof(sa));
	sa.sa_handler = sighandler;
	sigfillset(&sa.sa_mask);

	sigaction(SIGCHLD, &sa, NULL);
	sigaction(SIGTERM, &sa, NULL);
	sigaction(SIGINT, &sa, NULL);
	sigaction(SIGQUIT, &sa, NULL);
	sigaction(SIGHUP, &sa, NULL);

	signal(SIGPIPE, SIG_IGN);
	signal(SIGTSTP, SIG_IGN);
}

static void run_init(void)
{
	const char *prog = "/sbin/openrc-init";
	if (access(prog, X_OK) == 0) {
		execl(prog, prog, NULL);
		fprintf(stderr, "init: failed to exec %s: %s\n", prog, strerror(errno));
		return;
	}

	prog = "/etc/rc.init";
	if (access(prog, X_OK) == 0) {
		execl(prog, prog, NULL);
		fprintf(stderr, "init: failed to exec %s: %s\n", prog, strerror(errno));
		return;
	}

	prog = getenv("SHELL");
	if (!prog) prog = "/bin/sh";
	execl(prog, prog, NULL);
	fprintf(stderr, "init: failed to exec %s: %s\n", prog, strerror(errno));
}

int main(void)
{
	mount_fs();
	setup_signals();

	pid_t pid = fork();
	if (pid == 0) {
		setsid();
		run_init();
		_exit(1);
	}

	while (running) {
		int status;
		pid_t done = waitpid(-1, &status, 0);
		if (done < 0 && errno == ECHILD)
			pause();
	}

	reboot(RB_HALT_SYSTEM);
	return 0;
}
