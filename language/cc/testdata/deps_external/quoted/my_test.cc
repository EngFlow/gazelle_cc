#include "gtest/gtest.h"
#include "absl/algorithm/algorithm.h"
#include "absl/hash/hash.h"
#include "absl/log/log.h"
#include "boost/thread.hpp"

TEST(HelloTest, BasicAssertions) {
  EXPECT_STRNE("hello", "world");
  EXPECT_EQ(7 * 6, 42);
}