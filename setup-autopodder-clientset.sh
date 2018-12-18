# ROOT_PACKAGE :: the package (relative to $GOPATH/src) that is the target for code generation
ROOT_PACKAGE="github.com/morvencao/kube-auto-podder"
# CUSTOM_RESOURCE_NAME :: the name of the custom resource that we're generating client code for
CUSTOM_RESOURCE_NAME="autopodder"
# CUSTOM_RESOURCE_VERSION :: the version of the resource
CUSTOM_RESOURCE_VERSION="v1"

# retrieve the code-generator scripts and bins
go get -u k8s.io/code-generator/...

# run the code-generator entrypoint script
$GOPATH/src/k8s.io/code-generator/generate-groups.sh all "$ROOT_PACKAGE/pkg/client" "$ROOT_PACKAGE/pkg/apis" "$CUSTOM_RESOURCE_NAME:$CUSTOM_RESOURCE_VERSION"

# view the newly generated files
# morvencao$ tree pkg/client
# pkg/client
# |-- clientset
# |   `-- versioned
# |       |-- clientset.go
# |       |-- doc.go
# |       |-- fake
# |       |   |-- clientset_generated.go
# |       |   |-- doc.go
# |       |   `-- register.go
# |       |-- scheme
# |       |   |-- doc.go
# |       |   `-- register.go
# |       `-- typed
# |           `-- autopodder
# |               `-- v1
# |                   |-- autopodder.go
# |                   |-- autopodder_client.go
# |                   |-- doc.go
# |                   |-- fake
# |                   |   |-- doc.go
# |                   |   |-- fake_autopodder.go
# |                   |   `-- fake_autopodder_client.go
# |                   `-- generated_expansion.go
# |-- informers
# |   `-- externalversions
# |       |-- autopodder
# |       |   |-- interface.go
# |       |   `-- v1
# |       |       |-- autopodder.go
# |       |       `-- interface.go
# |       |-- factory.go
# |       |-- generic.go
# |       `-- internalinterfaces
# |           `-- factory_interfaces.go
# `-- listers
#     `-- autopodder
#         `-- v1
#             |-- autopodder.go
#             `-- expansion_generated.go

# 16 directories, 22 files
