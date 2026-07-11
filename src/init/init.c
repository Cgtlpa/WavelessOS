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

static volatile sig_atomic_t running = 1;
static volatile sig_atomic_t child_died = 0;

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
		child_died = 1;
		while (waitpid(-1, NULL, WNOHANG) > 0);
		break;
	case SIGTERM:
	case SIGINT:
	case SIGQUIT:
		running = 0;
		break;
	}
}

static void setup_signals(void)
{
	struct sigaction sa;
	memset(&sa, 0, sizeof(sa));

	sa.sa_handler = sighandler;
	sigemptyset(&sa.sa_mask);
	sigaddset(&sa.sa_mask, SIGCHLD);
	sigaddset(&sa.sa_mask, SIGTERM);
	sigaddset(&sa.sa_mask, SIGINT);
	sigaddset(&sa.sa_mask, SIGQUIT);
	sa.sa_flags = SA_RESTART | SA_NOCLDSTOP;

	sigaction(SIGCHLD, &sa, NULL);
	sigaction(SIGTERM, &sa, NULL);
	sigaction(SIGINT, &sa, NULL);
	sigaction(SIGQUIT, &sa, NULL);

	sa.sa_handler = SIG_IGN;
	sigemptyset(&sa.sa_mask);
	sa.sa_flags = 0;
	sigaction(SIGHUP, &sa, NULL);
	sigaction(SIGPIPE, &sa, NULL);
	sigaction(SIGTSTP, &sa, NULL);
}

static void run_script(const char *path)
{
	execl(path, path, NULL);
	fprintf(stderr, "init: failed to exec %s: %s\n", path, strerror(errno));
}

int main(void)
{
	mount_fs();
	setup_signals();

	const char *init_script = "/etc/rc.init";
	if (access(init_script, X_OK) != 0)
		init_script = NULL;

	const char *shell = getenv("SHELL");
	if (!shell) shell = "/bin/sh";

	while (running) {
		child_died = 0;

		pid_t pid = fork();
	if (pid == 0) {
		setsid();
		if (init_script) {
			run_script(init_script);
			fprintf(stderr, "init: init script %s failed, falling back to shell\n", init_script);
		}
		run_script(shell);
		_exit(1);
	}

		if (pid < 0) {
			perror("init: fork");
			sleep(1);
			continue;
		}

		while (running && !child_died)
			pause();

		if (!running)
			break;

		while (waitpid(-1, NULL, WNOHANG) > 0);
	}

	sync();
	reboot(RB_HALT_SYSTEM);
	return 0;
}
