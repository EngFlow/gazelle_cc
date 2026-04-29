#include "alone.h"
#include "shared_with_generated.h"
#include "shared_with_source.h"

static_assert(ALONE == 42);
static_assert(SHARED_WITH_GENERATED == 42);
static_assert(SHARED_WITH_SOURCE == 42);

int main() {
  return 0;
}
