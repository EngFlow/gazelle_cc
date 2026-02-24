#include "grpc_only/example_service.grpc.pb.h"
#include "proto_and_grpc/example_service.grpc.pb.h"
#include "proto_and_grpc/example_service.pb.h"
#include "proto_only/message.pb.h"

using Alias1 = grpc_only::ExampleService;
using Alias2 = proto_and_grpc::ExampleService;
using Alias3 = proto_and_grpc::Message;
using Alias4 = proto_only::Message;
