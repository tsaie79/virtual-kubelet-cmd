# Enhancements in this Branch
This branch introduces the following enhancements:
- [x] Enables the use of `configMap` and `secret` as volume types for script storage during pod launch.
- [x] Implements `volumes` in the pod to manage the usage of `configMap` and `secret`.
- [x] Employs `volumeMounts` to relocate scripts to the `mountPath`.
- [x] Utilizes `command` and `args` for script execution.
- [x] Supports `env` for passing environment variables to the scripts within a container.


# Use configMap as volume type for script storage
- The goal is to use `configMap` to define the user's job and support scripts. Any `configMap` in `volumes` will be created as a script by the vk. The location of the script is defined by the `mountPath` in `volumeMounts`.
- The root path of the `mountPath` is `$HOME/$podName/containers/$containerName`.

# Run the scripts in the container
- The `image` has the same name as the name in the one of the `volumeMounts` in the container.
- The `command` and `args` are used to execute the scripts in the container.
- The `pgid` is used to manage the process group of the scripts in the container. It is located in the `$HOME/$podName/containers/$containerName/pgid`.

For example, take the following `configMap.yaml` and `pod.yaml` as an example:
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: direct-stress
data:
  stress.sh: |
    #!/bin/bash
    stress --timeout $1 --cpu $2 # test memory
---
apiVersion: v1
kind: Pod
metadata:
  name: p1
  labels:
    app: new-test-pod
spec:
  containers:
    - name: c1
      image: direct-stress # this name should be the same as the name in the volumeMounts
      command: ["bash"]
      args: ["300", "2"] # the first argument is the timeout, and the second argument is the cpu number as defined in the stress.sh
      volumeMounts:
        - name: direct-stress
          mountPath: stress/job1 # the root path of the mountPath is $HOME/p1/containers/c1
  volumes:
    - name: direct-stress 
      configMap:
        name: direct-stress
```

# Run pod with virtual-kubelet nodes
- To run pod with `virtual-kubelet` nodes, the labels in `nodeSelector` and `tolerations` are required. 
```yaml
nodeSelector:
    kubernetes.io/role: agent
tolerations:
  - key: "virtual-kubelet.io/provider"
    value: "mock"
    effect: "NoSchedule"
```

# Affinity for pods in virtual-kubelet nodes
- `jiriaf.nodetype`, `jiriaf.site`, and `jiriaf.alivetime` are used to define the affinity of the virtual-kubelet nodes. These labels are defined as the environment variables `JIRIAF_NODETYPE`, `JIRIAF_SITE`, and `JIRIAF_WALLTIME` in the `start.sh` script. 
- Notice that if `JIRIAF_WALLTIME` is set to `0`, the `jiriaf.alivetime` will not be defined and the affinity will not be used.

```yaml
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: jiriaf.nodetype
            operator: In
            values:
            - "cpu"
          - key: jiriaf.site
            operator: In
            values:
            - "mylin"
          - key: jiriaf.alivetime # if JIRIAF_WALLTIME is set to 0, this label should not be defined.
            operator: Gt
            values:
            - "10"
```




# Key scripts
The main control of the vk is in the following files:
- cmd/virtual-kubelet/internal/provider/mock/mock.go
- cmd/virtual-kubelet/internal/provider/mock/command.go
- cmd/virtual-kubelet/internal/provider/mock/volume.go


# References
- [virtual-kubelet](https://github.com/virtual-kubelet/virtual-kubelet)
- [systemk](https://github.com/virtual-kubelet/systemk)
- [virtual-kubelet-cmd](https://github.com/tsaie79/virtual-kubelet-cmd)