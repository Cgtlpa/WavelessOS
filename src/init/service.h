#ifndef SERVICE_H
#define SERVICE_H

int svc_start(const char *name);
int svc_stop(const char *name);
int svc_restart(const char *name);
int svc_status(const char *name);
int svc_list(void);

#endif
