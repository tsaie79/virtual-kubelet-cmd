# Configuration for starting virtual-kubelet-cmd
## Environment variables
| Environment Variable      | Description |
| ----------- | ----------- |
| `MAIN`      | Main workspace directory       |
| `VK_PATH`   | Path to the directory containing the apiserver files     |
| `VK_BIN`    | Path to the binary files       |
| `APISERVER_CERT_LOCATION`| Location of the apiserver certificate |
| `APISERVER_KEY_LOCATION` | Location of the apiserver key |
| `KUBECONFIG` | Points to the location of the Kubernetes configuration file, which is used to connect to the Kubernetes API server. By default, it's located at `$HOME/.kube/config`. |
| `NODENAME` | The name of the node in the Kubernetes cluster. |
| `VKUBELET_POD_IP` | The IP address of the VK that metrics server talks to. If the metrics server is running in a Docker container and VK is running on the same host, this is typically the IP address of the `docker0` interface. |
| `KUBELET_PORT` | The port on which the Kubelet service is running. The default port for Kubelet is 10250. This is for the metrics server and should be unique for each node. |
| `JIRIAF_WALLTIME` | Sets a limit on the total time that a node can run. It should be a multiple of 60 and is measured in seconds. If it's set to 0, there is no time limit. |
| `JIRIAF_NODETYPE` | Specifies the type of node that the job will run on. This is just for labeling purposes and doesn't affect the actual job. |
| `JIRIAF_SITE` | Used to specify the site where the job will run. This is just for labeling purposes and doesn't affect the actual job. |

## Using Shell Scripts to Launch
The `test-run/start.sh` script provides an example of how to initiate the VK. It does this by setting up specific environment variables.
```bash
#!/bin/bash
export MAIN="/workspaces/virtual-kubelet-cmd"
export VK_PATH="$MAIN/test-run/apiserver"
export VK_BIN="$MAIN/bin"

export APISERVER_CERT_LOCATION="$VK_PATH/client.crt"
export APISERVER_KEY_LOCATION="$VK_PATH/client.key"
export KUBECONFIG="$HOME/.kube/config"

export NODENAME="vk"
export VKUBELET_POD_IP="172.17.0.1"
export KUBELET_PORT="10255" 

export JIRIAF_WALLTIME="60" 
export JIRIAF_NODETYPE="cpu"
export JIRIAF_SITE="Local"

"$VK_BIN/virtual-kubelet" --nodename $NODENAME --provider mock --klog.v 3 > ./$NODENAME.log 2>&1 
```

# Executing Pods with Virtual-Kubelet-CMD Nodes
Pods and their respective containers can be executed on Virtual-Kubelet-CMD (VK) nodes. The table below provides a comparison between the functionalities of a VK node and a regular kubelet:

| Feature | Virtual-Kubelet-CMD | Regular Kubelet |
| ------- | ------------------- | --------------- |
| Container | Executes as a series of Linux processes | Runs as a Docker container |
| Image | Defined as a shell script | Defined as a Docker container image |

## Enhanced Features for Script Storage and Execution in Pods
| Feature | Description |
| ------- | ----------- |
| `configMap`/ `secret` | These are used as volume types for storing scripts during the pod launch process |
| `volumes` | This feature is implemented within the pod to manage the use of `configMap` and `secret` |
| `volumeMounts` | This feature is used to relocate scripts to the specified `mountPath`. The `mountPath` is defined as a relative path. Its root is structured as `$HOME/$podName/containers/$containerName` |
| `command` and `args` | These are utilized to execute scripts |
| `env` | This feature is supported for passing environment variables to the scripts running within a container |
| `image` | The `image` corresponds to a `volumeMount` in the container and shares the same name |

## Process Group Management in Containers Using `pgid` files
The `pgid` file is a feature used to manage the process group of a shell script running within a container. Each container has a unique `pgid` file to ensure process management. The `pgid` can be found at the following location: `$HOME/$podName/containers/$containerName/pgid`.


# Lifecycle of containers and pods




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