#ifndef SKID_NATIVE_H
#define SKID_NATIVE_H

#include <stdint.h>

void skid_native_run(void);
void skid_native_send(uint64_t request, const char *operation, const char *payload);
void skid_native_stop(void);
int32_t skid_native_register(const char *application_path);
int32_t skid_native_status(const char *application_path);
int32_t skid_native_stop_application(const char *application_path);

#endif
