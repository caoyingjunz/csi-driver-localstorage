#!/bin/bash

NameSpace="kube-system"                                           # 命名空间
UserName=pixiu_user                                               # 你的用户名，可以改为任意内容
ApiServerEndpoints=`awk '$0~/server/{print $NF}' ~/.kube/config`  # 从.kube/config 获取集群IP
ClusterName=kubernetes                                            # 自定义集群标志，可以改为任意内容
SecretName=ls-sa-secret                                           # secret 的名称

cd /etc/kubernetes/ || exit                                        # kubeconfig存储目录

# 获取 Secret 中的token
TOKEN=$(kubectl get secret ${SecretName} -n ${NameSpace} -o jsonpath='{.data.token}'| base64 --decode)
CA_CRT=$(kubectl get secret ${SecretName} -n ${NameSpace} -o jsonpath='{.data.ca\.crt}')

# 创建 kubeconfig
cat << EOF > kubeconfig
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${CA_CRT}
    server: ${ApiServerEndpoints}
  name: ${ClusterName}
contexts:
- context:
    cluster: ${ClusterName}
    namespace: ${NameSpace}
    user: ${UserName}
  name: my-context
current-context: my-context
users:
- name: ${UserName}
  user:
    token: ${TOKEN}
EOF

# 创建role角色设置权限
kubectl create clusterrolebinding ${UserName}-binding \
  --clusterrole=ls-external-provisioner-role \
  --user=${UserName}