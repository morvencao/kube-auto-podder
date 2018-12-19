# kube-auto-podder

A simple kubernetes controller that generates pod automatically.

**Note**: the source code is _verbosely_ commented, so the source is meant to be read and to teach

## What is this?

An example of a custom Kubernetes controller that's only purpose is to watch for the creation, updating, or deletion of all custom resource of type `AutoPodder` (in the all namespaces). This repo is created to get a deep dive of how Kubernetes controllers work and interact with the cluster and resources.

## Deployment

1. Install the Kubernetes command line interface `kubectl` and obtain the cluster configuration details.
2. Run the following commands to start the kube-auto-podder controller:
    ```
    $ git clone https://github.com/morvencao/kube-auto-podder.git
    $ cd kube-auto-podder
    $ ./setup-autopodder-clientset.sh
    $ go build
    $ ./kube-auto-podder --kubeconfig ${KUBE_CONFIG_PATH} // ${KUBE_CONFIG_PATH} is $HOME/.kube/config be default
    ```
3. Deplicate a new terminal and run the following command to trigger the kube-auto-podder controller:
    ```
    $ kubectl create -f deployment/crd.yaml
    $ kubectl create -f deployment/example-crd.yaml
    ```
4. Verify that pod that contains `nginx:alpine` container has been automatically created:
    ```
    $ kubectl get pod | grep nginx
    ```
