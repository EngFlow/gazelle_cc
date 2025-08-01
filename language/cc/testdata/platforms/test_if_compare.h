#include "shared/api.h"
#if OS_WINDOWS // Explicit from BUILd.in
  #include "select/win.h"
#elif TARGET_OS_OSX // Implicit populated by platform defaults
    #include "select/macos.h"
#endif

#if PTR_SIZE >= 64
  #include "select/64bits.h"
#endif

#if PTR_SIZE < 64 
  #include "select/32bits.h"
#endif

#if !unix
  #include "select/non_unix.h"
#endif