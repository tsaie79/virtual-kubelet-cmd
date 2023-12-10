# Enhancements in this Branch
This branch introduces the following enhancements:
- [x] Enables the use of `configMap` and `secret` as volume types for script storage during pod launch.
- [x] Implements `volumes` in the pod to manage the usage of `configMap` and `secret`.
- [x] Employs `volumeMounts` to relocate scripts to the `mountPath`.
- [x] Utilizes `command` and `args` for script execution.
- [x] Supports `env` for passing environment variables to the scripts within a container.


# How to use configMap as a volume type to write scripts
The goal is to use `configMap` to define the user's job and support scripts. Any `configMap` in `Volumes` will be created as a script by the vk. However, only those in `volumeMounts` AND with file names ended with `.job` will be executed by the vk. The command to execute the scripts is defined in the `command` field of the `container`.

Since each container has its own control of resources, one should only include one `job-script.job` in a single container. The support scripts, such as `support.sh`, can be included in the same container, but it is recommended to be light-weighted. Multiple jobs can be executed in a single pod by adding multiple containers.

## The thumb rules of preparing the scripts for the vk
1. The `configMap` in `Volumes` will be created as a script by the vk.
2. The volume in `volumeMounts` AND with file names ended with `.job` will be executed by the vk.
4. The 
3. The command to execute the scripts is defined in the `command` field of the `container`.




The mounted path is defined automatically by the vk, following the rule of:
```text
$HOME/$pod_name/$type_of_volume/$volume_name/$script_named
``` 
where, 
1. `$type_of_volume` is either `configmaps` or `secrets`.
2. `$volume_name` is the name of the `Volumes:Name`.
3. `$script_name` is the name of the `ConfigMap:Data` or `Secret:Data`.

For example, take the following `pod.yaml` as an example:
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: user1
  namespace: default
data:
  hello.job: |
    #!/bin/bash
    echo "Hello World" > ~/test1.txt && sleep 10
---
apiVersion: v1
kind: Pod
metadata:
  name: multijobs
spec:
  containers:
  - name: job-user1
    image: user1 #required but no effect
    command: ["/bin/bash"] #required but no effect
    volumeMounts:
    - name: user1
      mountPath: "/no/effect" #required but no effect

  volumes:
    - name: user1
      configMap:
        name:  user1
```
The mounted path is: 
```text
$HOME/multijobs/configmaps/user1/hello.sh
```

# How user's workload is executed
Users should use `configMap` to define their workloads. When the pod containing the `configMap` is launched, the vk will create a `bash` script, and then execute it by the command:
```bash
bash $HOME/$pod_name/$type_of_volume/$volume_name/$script_name
```

# How to execute the multiple jobs in a single pod
One can mount multiple `configMap` to a single pod, and then execute them within the same pod. The commands will be executed parallelly by the go routines.

## Definitions of `configMap`, `volume`, and `container`:
| Kubernetes  | vk                   |
| ----------- | -------------------- |
| `configMap` | Job script / workdir |
| `volume`    |                      |
| `container` | User                 |


## Example of multiple jobs in a single pod

There are two job scripts, `job1.job` and `job2.job`, and one support script, `support.sh`. The `job1.job` and `job2.job` are mounted to the containers `job1` and `job2`, respectively. The `support.sh` is mounted the container `job1`. The `job1` and `job2` are executed parallelly, and the `support.sh` is executed after `job1` is finished.



```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: job-1
  namespace: default
data:
  job1.job: |
     sleep 1
     sleep 2
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: job-2
  namespace: default
data :
  job2.job: |
     sleep 3
     sleep 4
     
---

kind: ConfigMap
apiVersion: v1
metadata:
  name: support
  namespace: default
data:
  support.sh: |
    echo "support" > ~/support.txt && sleep 10

---

apiVersion: v1
kind: Pod
metadata:
  name: multijobs
spec:
  containers:
  - name: job1
    image: job1
    command: ["/bin/bash"]
    volumeMounts:
    - name: job-1
      mountPath: /job1
    - name: support
      mountPath: /support

  - name: job2
    image: job2
    command: ["/bin/bash"]
    volumeMounts:
    - name: job-2
      mountPath: /job2


  volumes:
    - name: job-1
      configMap:
        name:  job-1
    - name: job-2
      configMap:
        name:  job-2
    - name: support
      configMap:
        name:  support

  nodeSelector:
    kubernetes.io/role: agent
  tolerations:
  - key: "virtual-kubelet.io/provider"
    value: "mock"
    effect: "NoSchedule"


---

---
```
Notice that the `command`, `image`, and `mountPath` are required, but they have no effect.


# Key scripts
The main control of the vk is in the following files:
- cmd/virtual-kubelet/internal/provider/mock/mock.go
- cmd/virtual-kubelet/internal/provider/mock/command.go
- cmd/virtual-kubelet/internal/provider/mock/volume.go


# References
- [virtual-kubelet](https://github.com/virtual-kubelet/virtual-kubelet)
- [systemk](https://github.com/virtual-kubelet/systemk)
- [virtual-kubelet-cmd](https://github.com/tsaie79/virtual-kubelet-cmd)