#include "shared/api.h"
#if PTR_SIZE == 64
  #include "select/64bits.h"
#elif PTR_SIZE == 32 
  #include "select/32bits.h"
#endif 

#if TARGET_OS_OSX
  #include "select/macos.h"
#endif

#if defined __unix__ && ! TARGET_OS_UNIX
  #include "select/unix.h"
#endif 
