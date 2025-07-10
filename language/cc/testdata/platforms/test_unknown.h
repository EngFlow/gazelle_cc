#if defined UNKNOWN__FLAG
  #include "shared/api.h"
#endif

#if defined(_WIN32) && UNKNOWN_WINDOWS_FLAG
  #include "select/win.h"
#endif

#if defined(__APPLE__) && !CUSTOM_OSX_MACRO
  #include "select/macos.h"
#endif

#if unix && defined SOME_UNIX_MACRO
  #include "select/unix.h"
#endif

