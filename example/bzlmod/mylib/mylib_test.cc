#include <gtest/gtest.h>
#include <string>
#include "mylib.h"

TEST(AdditionTest, HandlesVariousInputs) {
    EXPECT_EQ(mylib::add(1, 2), 3);
    EXPECT_EQ(mylib::add(-5, 5), 0);
    EXPECT_EQ(mylib::add(0, 0), 0);
}

TEST(GreetTest, ProducesCorrectGreeting) {
    EXPECT_EQ(mylib::greet("Alice"), "Hello, Alice!");
    EXPECT_EQ(mylib::greet("Bob"), "Hello, Bob!");
}
