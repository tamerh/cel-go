workspace(name = "cel_go")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository", "new_git_repository")

http_archive(
    name = "io_bazel_rules_go",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.18.6/rules_go-0.18.6.tar.gz"],
    sha256 = "f04d2373bcaf8aa09bccb08a98a57e721306c8f6043a2a0ee610fd6853dcde3d"
)

http_archive(
    name = "bazel_gazelle",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.17.0/bazel-gazelle-0.17.0.tar.gz"],
    sha256 = "3c681998538231a2d24d0c07ed5a7658cb72bfb5fd4bf9911157c0e9ac6a2687",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@bazel_gazelle//:deps.bzl", "go_repository")

# Do *not* call *_dependencies(), etc, yet.  See comment at the end.

go_repository(
  name = "org_golang_google_genproto",
  build_file_proto_mode = "disable",
  commit = "bd91e49a0898e27abb88c339b432fa53d7497ac0",
  importpath = "google.golang.org/genproto",
)

go_repository(
  name = "com_github_antlr",
  tag = "4.7.2",
  importpath = "github.com/antlr/antlr4",
)

git_repository(
  name = "com_google_cel_spec",
  commit = "3de74f02b6b58fba89a17c4ea0fe32efd560795c", #PR #95
  remote = "https://github.com/google/cel-spec.git",
)

# Required to use embedded BUILD.bazel file in googleapis/google/rpc
git_repository(
    name = "io_grpc_grpc_java",
    remote = "https://github.com/grpc/grpc-java.git",
    tag = "v1.22.1",
)

new_git_repository(
    name = "com_google_googleapis",
    remote = "https://github.com/googleapis/googleapis.git",
    commit = "980cdfa876e54b1db4395617e14037612af25466",
    build_file_content = """
load('@io_bazel_rules_go//proto:def.bzl', 'go_proto_library')

cc_proto_library(
    name = 'cc_rpc_status',
    deps = ['//google/rpc:status_proto'],
    visibility = ['//visibility:public'],
)

cc_proto_library(
    name = 'cc_rpc_code',
    deps = ['//google/rpc:code_proto'],
    visibility = ['//visibility:public'],
)

cc_proto_library(
    name = 'cc_expr_v1beta1',
    deps = [
        '//google/api/expr/v1beta1/eval_proto',
        '//google/api/expr/v1beta1/value_proto',
    ],
    visibility = ['//visibility:public'],
)

go_proto_library(
    name = 'rpc_status_go_proto',
    # TODO: Switch to the correct import path when bazel rules fixed.
    #importpath = 'google.golang.org/genproto/googleapis/rpc/status',
    importpath = 'github.com/googleapis/googleapis/google/rpc',
    proto = '//google/rpc:status_proto',
    visibility = ['//visibility:public'],
)

go_proto_library(
    name = 'expr_v1beta1_go_proto',
    importpath = 'google.golang.org/genproto/googleapis/api/expr/v1beta1',
    proto = '//google/api/expr/v1beta1',
    visibility = ['//visibility:public'],
    deps = ['@com_google_googleapis//:rpc_status_go_proto'],
)
"""
)

go_repository(
  name = "org_golang_google_grpc",
  importpath = "google.golang.org/grpc",
  tag = "v1.11.3",
  remote = "https://github.com/grpc/grpc-go.git",
  vcs = "git",
)

go_repository(
  name = "org_golang_x_text",
  importpath = "golang.org/x/text",
  tag = "v0.3.0",
  remote = "https://github.com/golang/text",
  vcs = "git",
)

# Run the dependencies at the end.  These will silently try to import some
# of the above repositories but at different versions, so ours must come first.
go_rules_dependencies()
go_register_toolchains()
gazelle_dependencies()
