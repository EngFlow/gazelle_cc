#include <iostream>
#include <string>

namespace {

const std::string STR{"#include <fmt/core.h>"};
const std::wstring WSTR{L"Hello, world! 😎😎😎"}; // This comment starts at line 7, column 48

const std::string RAW_STR{R"delim(
This is a raw string.
)delim"};

} // namespace
