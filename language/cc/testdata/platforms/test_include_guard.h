#ifndef TEST_INCLUDE_GUARD_H
#define TEST_INCLUDE_GUARD_H

#include "shared/api.h"

#if _WIN32
  #include "select/win.h"
#endif

#if __APPLE__
  #include "select/macos.h"
#endif

#if __unix__
  #include "select/unix.h"
#endif

#endif