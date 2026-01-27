#include "shared/api.h"
#if OS_WINDOWS 
  #include "select/win.h"
#endif 

#if TARGET_OS_OSX
  #include "select/macos.h"
#endif

#if __unix__ 
#include "select/unix.h"
#endif 
