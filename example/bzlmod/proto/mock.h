#include <iostream>
#include <string>

#include "mylib/mylib.h"
#include "proto/sample.pb.h"

#include "google/protobuf/util/time_util.h"
#include <google/protobuf/json/json.h>

int printMessage() {
  GOOGLE_PROTOBUF_VERIFY_VERSION;

  Foo foo;
  foo.set_name("MyFooObject");
  *foo.mutable_last_updated() =
      google::protobuf::util::TimeUtil::GetCurrentTime();
  auto *type = foo.mutable_type();
  type->set_name("example.Type");

  std::string json_output;
  google::protobuf::json::PrintOptions opts;
  opts.add_whitespace = true; // pretty-print
  auto status =
      google::protobuf::json::MessageToJsonString(foo, &json_output, opts);

  if (!status.ok()) {
    std::cerr << "JSON serialization failed: " << status.message() << "\n";
    return 1;
  }

  std::cout << "Foo as JSON:\n" << json_output << "\n";

  google::protobuf::ShutdownProtobufLibrary();
  return 0;
}
