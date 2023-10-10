
# Introduction

This provider is a type of Virtual Kubelet that translates Kubernetes commands into shell commands that can be run on the host. Instead of running a container, it runs shell commands directly on the host. This provider is based on the Virtual Kubelet (vk-mock) and is designed to run on resources where the container runtime is not directly accessible. We modified the `CreatePod` function in `mock.go` to execute the shell commands specified in the pod spec. Users can specify these commands in the pod spec, and they will be executed on the host shell.

# Pod Status

The status of the pod will be updated based on the status of the command that is executed. Here are the possible statuses:

- `CmdSucceeded`: The command has completed successfully, and the pod will be deleted.
- `CmdRunning`: The pod has been created, and the command is currently running.
- `CmdFailed`: The command has failed to run successfully. If you are using a UNIX pipe to run commands on the host from the container (e.g., `echo "cmd on host" > pipeline`), any errors from the command string ("cmd on host") will not be reflected in the pod status. Instead, you will need to check the `pipeline.out` file for the error message.

# Build Docker image for virtual-kubelet-cmd

Git repo: [vk-cmd](https://github.com/tsaie79/vk-cmd)

# Demo 

- Create virtual-kubelet

![image](images/create_vk.gif)

- Create pod with shell command `touch > ./out.txt` in the `test-run/job_pod_template.yaml`. The status of the pod will be updated to `CmdSucceeded` when the command is completed.

![image](images/cmd_succeeded.gif)

- Pod status `CmdRunning` when the command is running

![image](images/cmd_running.gif)

- Pod status `CmdFailed` when the command is not valid

![image](images/cmd_failed.gif)


# References
This package is based on the mock provider of [Virtual Kubelet](https://github.com/virtual-kubelet/virtual-kubelet).