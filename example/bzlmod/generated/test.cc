#include "generated/alone.h"
#include "generated/shared_with_generated.h"
#include "generated/shared_with_source.h"

#include <gtest/gtest.h>

TEST(GeneratedTest, Constants) {
    EXPECT_EQ(ALONE, 42);
    EXPECT_EQ(SHARED_WITH_GENERATED, 42);
    EXPECT_EQ(SHARED_WITH_SOURCE, 42);
}
