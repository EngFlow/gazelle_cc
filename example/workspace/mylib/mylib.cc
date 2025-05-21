#include <fmt/core.h>
#include "mylib.h"

namespace mylib {

int add(int a, int b) { return a + b; }

std::string greet(const std::string &name) {
  return fmt::format("Hello, {}!", name);
}

} // namespace mylib
