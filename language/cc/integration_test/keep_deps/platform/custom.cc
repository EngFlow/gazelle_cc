#if __ANDROID__
#   include "keep_deps/select/android.h"
#elif __QNX__
#   include "keep_deps/select/qnx.h"
#else
#   include "keep_deps/select/default.h"
#endif
