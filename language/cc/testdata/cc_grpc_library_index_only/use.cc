#include "grpc_only/example_service.grpc.pb.h"
#include "proto_and_grpc/example_service.grpc.pb.h"
#include "proto_and_grpc/example_service.pb.h"
#include "proto_only/message.pb.h"

using Service1 = grpc_only::ExampleService;
using Service2 = proto_and_grpc::ExampleService;
using Service3 = proto_only::Message;
