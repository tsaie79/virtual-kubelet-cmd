# Features of this branch
This branch is to support the following features:
- [x] Support `configMap` and `secret` to write scripts at which the pod is launched.
- [x] Use `configMap` and `secret` to define the user's workload. The `command`, `image`, and `mountPath` are required, but they have no effect. Mounted path is defined automatically by the vk.
- [x] Support multiple jobs in a single pod.


# How to use volume to mount configMap and secret (creating scripts)
The mounted path is defined automatically by the vk, following the rule of:
```text
$HOME/$pod_name/$type_of_volume/$volume_name/$script_name
``` 
where `$type_of_volume` is either `configmaps` or `secrets`, and `$volume_name` is the name of the `Volumes:Name`. The `$script_name` is the name of the `ConfigMap:Data` or `Secret:Data`. Notice the `MountPath` is not used in this case.


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
    image: user1 #no effect
    command: [""]
    volumeMounts:
    - name: user1
      mountPath: "/no/effect" 
  volumes:
    - name: user1
      configMap:
        name:  user1
```
The mounted path is: 
```text
$HOME/multijobs/~/jobs/configmaps/user1/hello.sh
```

# How user's workload is executed
Users should use `configMap` to define their workloads. When the pod containing the `configMap` is launched, the vk will create a `bash` script, and then execute it by the command:
```bash
bash $HOME/$pod_name/$type_of_volume/$volume_name/$script_name
```

# How to execute the multiple jobs in a single pod
One can mount multiple `configMap` to a single pod, and then execute them within the same pod. The commands will be executed parallelly. No wait is required.

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
kind: ConfigMap
apiVersion: v1
metadata:
  name: user2
  namespace: default
data:
  good.sh: |
    #!/bin/bash
    echo "I am good" > ~/test2.txt && sleep 60

---

apiVersion: v1
kind: Pod
metadata:
  name: multijobs
spec:
  containers:
  - name: job-user1
    image: user1
    command: [""]
    volumeMounts:
    - name: user1
      mountPath: "/no/effect" 

  - name: job-user2
    image: user2
    command: [""]
    volumeMounts:
    - name: user2
      mountPath: "/no/effect"

  volumes:
    - name: user1
      configMap:
        name:  user1

    - name: user2
      configMap:
        name:  user2
---
```
Notice that the `command`, `image`, and `mountPath` are required, but they have no effect.


# References
- [systemk](https://github.com/virtual-kubelet/systemk)