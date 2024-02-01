#!/bin/bash

export MAIN="/workspaces/virtual-kubelet-cmd"
export VK_PATH="$MAIN/test-run/apiserver"
export KUBECONFIG="$HOME/.kube/config"
export VKUBELET_POD_IP="172.17.0.1" # "10.250.64.71"
export APISERVER_CERT_LOCATION="$VK_PATH/client.crt"
export APISERVER_KEY_LOCATION="$VK_PATH/client.key"
export KUBELET_PORT="10255"
export NODENAME="vk"

export JIRIAF_WALLTIME="60" # set multiple of 60
export JIRIAF_NODETYPE="cpu"
export JIRIAF_SITE="Local"

# echo "{\"$NODENAME\": {\"cpu\": \"0\", \"memory\": \"0Gi\", \"pods\": \"0\"}}" > $HOME/.host-cfg.json

"$MAIN/virtual-kubelet-cmd" --nodename $NODENAME --provider mock --klog.v 3 > ./$NODENAME.log 2>&1 
# "$VK_BIN/virtual-kubelet" --nodename $NODENAME --provider mock --provider-config $HOME/.host-cfg.json --log-level debug --klog.v 3 > ./vk.log 2>&1 
# 