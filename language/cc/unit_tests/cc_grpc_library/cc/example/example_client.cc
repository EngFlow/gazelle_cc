#include <grpcpp/grpcpp.h>
#include <iostream>
#include <memory>
#include <string>

#include "proto/example/example_service.grpc.pb.h"

const std::string serverAddress("localhost:50051");

class ExampleServiceClient {
public:
    ExampleServiceClient(std::shared_ptr<grpc::Channel> channel)
        : stub_{example::ExampleService::NewStub(channel)} {}

    bool CallTestMethod() {
        grpc::ClientContext context;
        google::protobuf::Empty request;
        google::protobuf::Empty response;

        grpc::Status status = stub_->Call(&context, request, &response);

        if (status.ok()) {
            std::cout << "example::ExampleService::Stub::Call() succeeded" << std::endl;
            return true;
        } else {
            std::cout << "example::ExampleService::Stub::Call() failed: " << status.error_code() << ": " << status.error_message() << std::endl;
            return false;
        }
    }

private:
    std::unique_ptr<example::ExampleService::Stub> stub_;
};

int main(int argc, char** argv) {
    auto channel = grpc::CreateChannel(serverAddress, grpc::InsecureChannelCredentials());

    ExampleServiceClient client{channel};

    std::cout << "Connecting to server at " << serverAddress << std::endl;
    bool success = client.CallTestMethod();

    return success ? 0 : 1;
}
