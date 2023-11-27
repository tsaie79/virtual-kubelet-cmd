# Features of this branch
This branch is to support the following features:
- [x] Support `configMap` and `secret` as volume types to write scripts at which the pod is launched.
- [x] Use `configMap` and `secret` to define the user's workload. The `command`, `image`, and `mountPath` are required, but they have no effect. Mounted path is defined automatically by the vk.
- [x] Support multiple jobs in a single pod.


# How to use volume to mount configMap and secret (creating scripts)
The mounted path is defined automatically by the vk, following the rule of:
```text
$HOME/$pod_name/$type_of_volume/$volume_name/$script_name
``` 
where, 
1. `$type_of_volume` is either `configmaps` or `secrets`.
2. `$volume_name` is the name of the `Volumes:Name`.
3. `$script_name` is the name of the `ConfigMap:Data` or `Secret:Data`. To be a valid script, the name should end with `.sh`.
4. Notice the `MountPath` is not used in this case.


For example, take the following `pod.yaml` as an example:
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: user1
  namespace: default
data:
  hello.sh: |
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
    command: [""] #required but no effect
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

There are two users, `u1` and `u2`. User `u1` has two jobs, `u1-j1` and `u1-j2`. User `u2` has one job, `u2-j1`. Jobs are defined by `configMap` and `volume`, while `container` is defined by user.
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: u1-j1
  namespace: default
data:
  u1-j1.sh: |
    #!/bin/bash
     sleep 1
     sleep 2
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: u1-j2
  namespace: default
data :
  u1-j2.sh: |
    #!/bin/bash
     sleep 3
     sleep 4
     
---

kind: ConfigMap
apiVersion: v1
metadata:
  name: u2-j1
  namespace: default
data:
  u2-j1.sh: |
    #!/bin/bash
     sleep 5
     sleep 6

---

apiVersion: v1
kind: Pod
metadata:
  name: multijobs
spec:
  containers:
  - name: u1
    image: u1
    command: [""]
    volumeMounts:
    - name: u1-j1
      mountPath: u1-j1
    - name: u1-j2
      mountPath: u1-j2

  - name: u2
    image: u2
    command: [""]
    volumeMounts:
    - name: u2-j1
      mountPath: u2-j1

  volumes:
    - name: u1-j1
      configMap:
        name: u1-j1
    
    - name: u1-j2
      configMap:
        name: u1-j2

    - name: u2-j1
      configMap:
        name:  u2-j1

        
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