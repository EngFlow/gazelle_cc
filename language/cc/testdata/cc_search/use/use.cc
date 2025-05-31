#include "extra/prefix/prefix.h"
#include "self/self.h"
#include "stripped_prefix_abs.h"
#include "stripped_prefix_rel.h"
#include "extra/stripped_include_prefix_and_prefix.h"

void use() {
	prefix();
	self();
	stripped_prefix_abs();
	stripped_prefix_rel();
}
