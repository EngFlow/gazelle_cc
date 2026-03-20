package(default_visibility = ["//visibility:public"])

platform(
    name = "macos_arm64",
    constraint_values = [
        "@platforms//os:macos",
        "@platforms//cpu:arm64",
        "@rules_go//go/toolchain:cgo_off",
    ],
    exec_properties = {
        "Pool": "macos_arm_m2",
    },
)
