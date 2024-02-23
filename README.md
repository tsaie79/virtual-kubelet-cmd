# Configuration for starting virtual-kubelet-cmd
## Environment variables
| Environment Variable      | Description |
| ----------- | ----------- |
| `MAIN`      | Main workspace directory       |
| `VK_PATH`   | Path to the directory containing the apiserver files     |
| `VK_BIN`    | Path to the binary files       |
| `APISERVER_CERT_LOCATION`| Location of the apiserver certificate |
| `APISERVER_KEY_LOCATION` | Location of the apiserver key |
| `KUBECONFIG` | Path to the kubeconfig file |
| `NODENAME` | Name of the VK node |
| `VKUBELET_POD_IP` | IP of the apiserver. If the apiserver is running in a Docker container, this should be the IP of `docker0` |
| `KUBELET_PORT` | Port used for communication with the apiserver |
| `JIRIAF_WALLTIME` | Walltime for the VK nodes |
| `JIRIAF_NODETYPE` | Node type for the VK nodes |
| `JIRIAF_SITE` | Site for the VK nodes |


For example, the `start.sh` script is used to start the VK with the following environment variables:
```bash
#!/bin/bash
export MAIN="/workspaces/virtual-kubelet-cmd"
export VK_PATH="$MAIN/test-run/apiserver"
export VK_BIN="$MAIN/bin"

# the following environment variables are used to define the apiserver
export APISERVER_CERT_LOCATION="$VK_PATH/client.crt"
export APISERVER_KEY_LOCATION="$VK_PATH/client.key"
export KUBECONFIG="$HOME/.kube/config"

# the following environment variables are used to define the VK node
export NODENAME="vk"
export VKUBELET_POD_IP="172.17.0.1" # the ip of the apiserver, if the apiserver is running in the docker container, the ip should be the ip of the docker0
export KUBELET_PORT="10255" # this port is used to communicate with the apiserver

# the following environment variables are used to define the affinity of the VK nodes
export JIRIAF_WALLTIME="60" 
export JIRIAF_NODETYPE="cpu"
export JIRIAF_SITE="Local"

# the command to start the VK
"$VK_BIN/virtual-kubelet" --nodename $NODENAME --provider mock --klog.v 3 > ./$NODENAME.log 2>&1 
```


# Features of the branch
- Enables the use of `configMap` and `secret` as volume types for script storage during pod launch.
- Implements `volumes` in the pod to manage the usage of `configMap` and `secret`.
- Employs `volumeMounts` to relocate scripts to the `mountPath`.
- Utilizes `command` and `args` for script execution.
- Supports `env` for passing environment variables to the scripts within a container.


# Use configMap as volume type for script storage
- Any `configMap` in `volumes` will be created as a script by the vk. Then, it will be copied to the `mountPath` in the container.
- Root path of the `mountPath` is `$HOME/$podName/containers/$containerName`.

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

# Run pod with VK nodes
- To run pod with VK nodes, the labels in `nodeSelector` and `tolerations` are required. 
```yaml
nodeSelector:
    kubernetes.io/role: agent
tolerations:
  - key: "virtual-kubelet.io/provider"
    value: "mock"
    effect: "NoSchedule"
```

# Affinity for pods in VK nodes
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

- To add more labels to the VK nodes, modify `ConfigureNode` in `internal/provider/mock/mock.go`.

# Metrics Server Deployment

The Metrics Server is a tool that collects and provides resource usage data for nodes and pods within a Kubernetes cluster. The necessary deployment configuration is located in the `metrics-server/components.yaml` file.

To deploy the Metrics Server, execute the following command:

```bash
kubectl apply -f metrics-server/components.yaml
```




# Key scripts
The main control of the vk is in the following files:
- `internal/provider/mock/mock.go`
- `internal/provider/mock/command.go`
- `internal/provider/mock/volume.go`


# References
- [virtual-kubelet](https://github.com/virtual-kubelet/virtual-kubelet)
- [systemk](https://github.com/virtual-kubelet/systemk)