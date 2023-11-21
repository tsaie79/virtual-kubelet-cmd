#!/bin/bash

export MAIN="/workspaces/add-volume"
export VK_PATH="$MAIN/test-run/apiserver"
export VK_BIN="$MAIN/bin"
export KUBECONFIG="$HOME/.kube/config"
export VKUBELET_POD_IP="10.250.64.71"
export APISERVER_CERT_LOCATION="$VK_PATH/client.crt"
export APISERVER_KEY_LOCATION="$VK_PATH/client.key"
export KUBELET_PORT="10250"
export NODENAME="vk-config-test"


echo "{\"$NODENAME\": {\"cpu\": \"0\", \"memory\": \"0Gi\", \"pods\": \"0\"}}" > $HOME/.host-cfg.json

"$VK_BIN/virtual-kubelet" --nodename $NODENAME --provider mock --provider-config $HOME/.host-cfg.json --log-level debug --klog.v 3 > ./vk.log 2>&1 