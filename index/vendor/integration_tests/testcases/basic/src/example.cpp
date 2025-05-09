#include <cstdlib>
#include "lib_a/foo.h"
#include "bar.h"
#include "third_party/c/baz.h"


int main() {
    foo();
    bar();
    baz();
    return EXIT_SUCCESS;
}