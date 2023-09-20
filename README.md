# BASH Commands Provider for Virtual Kubelet

This provider is a Virtual Kubelet that translates the commands from Kubernetes to the host shell commands. It is not running a container, but running shell commands on the host. This is based on the Virtual Kubelet (vk-mock) and is designed for running on various resources where a user can't reach the container runtime directly.

We modified the CreatePod in mock.go to run the shell commands. Users can specify the commands in the pod spec. The commands will be executed in the host shell. The status of the pod will be updated to `CmdCompleted` when the commands are completed.