#include "shared/api.h"
#if defined OS_WINDOWS 
  #include "select/win.h"
#elif defined(__APPLE__)
    #include "select/macos.h"
#elif !defined(unix)
  #error "Unknown platform"
#else 
  #include "select/unix.h"
#endif
