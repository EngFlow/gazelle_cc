#if __ANDROID__
#   include "select/android.h"
#elif __QNX__
#   include "select/qnx.h"
#else
#   include "select/default.h"
#endif
