#include "mylib/mylib.h"
#include <iostream>
#include <ostream>

int main(int argc, char **argv) {
  std::cout << mylib::greet("Workspace") << std::endl;
  return 0;
}
