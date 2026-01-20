#include <atomic>
#include <chrono>
#include <csignal>
#include <grpcpp/grpcpp.h>
#include <iostream>
#include <memory>
#include <string>
#include <thread>

#include "proto/example/example_service.grpc.pb.h"

std::unique_ptr<grpc::Server> server;
std::atomic_bool serverRunning{true};
const std::string serverAddress("0.0.0.0:50051");

class ExampleServiceImpl final : public example::ExampleService::Service {
    grpc::Status Call(grpc::ServerContext* context, const google::protobuf::Empty* request, google::protobuf::Empty* response) override {
        std::cout << "example::ExampleService::Service::Call() called from: " << context->peer() << std::endl;
        return grpc::Status::OK;
    }
};

void SignalHandler(int signal) {
    std::cout << "\nReceived SIGINT, shutting down gracefully..." << std::endl;
    serverRunning.store(false);
}

void ServerShutdownHandler(grpc::Server *server) {
    while (serverRunning.load()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    server->Shutdown();
}

void RunServer() {
    ExampleServiceImpl service;

    server = grpc::ServerBuilder{}
        .AddListeningPort(serverAddress, grpc::InsecureServerCredentials())
        .RegisterService(&service)
        .BuildAndStart();

    std::cout << "Server listening on " << serverAddress << std::endl;

    std::thread shutdownHandler(ServerShutdownHandler, server.get());
    server->Wait();
    shutdownHandler.join();
}

int main(int argc, char** argv) {
    std::signal(SIGINT, SignalHandler);
    RunServer();
    return 0;
}
