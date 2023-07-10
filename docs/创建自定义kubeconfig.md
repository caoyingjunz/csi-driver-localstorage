# 通过ServiceAccount创建kubeconfig文件

前置知识：
> 在 kubernetes1.24 之前，创建 ServiceAccount 会自动生成对应的Secret，在 Secret 中会存储对应的 ca.crt 和 token。而在kubernetes1.24 之后，创建 ServiceAccount 不会自动创建 Secret，需要手动创建 Secret，并从其中获取 ca.crt 和 token，然后再生成 kubeconfig 文件。

**本文针对 `kubernetes1.24` 之前和 `kubernetes1.24` 之后的版本，提供了两种解决方案，具体步骤如下**：

1、查看当前 `kubernetes` 版本，判断版本号是否大于 `kubernetes 1.24`

```shell
# 查看kubernetes 版本
kubectl version
```

2、首先需要先检查一下是否部署 `csi-ls-node-sa` 这个 `ServiceAccount` 如果没有需要先创建，如果有则跳过此步骤

```shell
# 查看是否部署csi-ls-node-sa这个ServiceAccount
kubectl get sa -n kube-system

NAME                                 SECRETS   AGE
csi-ls-node-sa                       1         1m

# 没有的话创建(其中的v1.0.1为每个版本的版本号，根据最新版本进行调整)
kubectl apply -f deploy/v1.0.1/ls-rbac.yaml
```

## 方案1：当前版本小于 `kubernetes 1.24`

1、跳转到用户目录`~`，创建 `gen_kubeconfig.sh`，文件内容如下：

```shell
cd ~ # 跳转到用户目录或者拥有.kube的目录
# 创建gen_kubeconfig.sh，将以下内容填入:

#!/bin/bash

# 设置 ServiceAccount 的名称和命名空间
SERVICEACCOUNT_NAME="csi-ls-node-sa"                              # sa的名称
NAMESPACE="kube-system"                                           # 命名空间
UserName=pixiu_user                                               # 你的用户名，可以改为任意内容
ApiServerEndpoints=`awk '$0~/server/{print $NF}' ~/.kube/config`  # 从.kube/config获取集群IP
ClusterName=kubernetes                                            # 自定义集群标志，可以改为任意内容
mkdir -p /etc/kubernetes/                                         # kubeconfig存储目录
cd /etc/kubernetes/

# 获取 ServiceAccount 的 Secret 的名称
SECRET_NAME=\$(kubectl get serviceaccount \${SERVICEACCOUNT_NAME} -n \${NAMESPACE} -o jsonpath='{.secrets[0].name}')

# 获取 `Secret` 中的证书和 token，token 需要 base64 解码
TOKEN=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.token}'| base64 --decode)
CA_CRT=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.ca\.crt}')

# 创建kubeconfig
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
    namespace: ${NAMESPACE}
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
```

2、 执行 `gen_kubeconfig.sh` 生成 `kubeconfig` 文件

```shell
sh gen_kubeconfig.sh
```

3、 查看 `/etc/kubernetes` 是否存在 `kubeconfig` 文件

```shell
cd /etc/kubernetes && ls

admin.conf  controller-manager.conf  kubeconfig  kubelet.conf  ls-scheduler-extender.yaml  manifests  pki  scheduler.conf
```

## 方案2：当前 `kubernetes` 版本大于`kubernetes 1.24`

1、手动创建 `Secret` 并与 `ServiceAccount` 进行关联

```yaml
# 创建ls-account-secret.yaml，将以下内容填入：
apiVersion: v1
kind: Secret
metadata:
  name: ls-account-secret
  namespace: kube-system
  annotations:
    kubernetes.io/service-account.name: "csi-ls-node-sa"   # 这里填写serviceAccountName
type: kubernetes.io/service-account-token
```

2、 部署Secret

```shell
kubectl apply -f ls-account-secret.yaml
```

3、 查看Secret是否创建成功

```shell
kubectl get secret ls-account-secret -n kube-system

NAME                TYPE                                  DATA   AGE
ls-account-secret   kubernetes.io/service-account-token   3      55m
```

4、 查看`ServiceAccount` 的信息，看看 `Token` 中有没有关联上上面创建的 `Secret`

```shell
kubectl describe sa  csi-ls-node-sa -n kube-system

Name:                csi-ls-node-sa
Namespace:           kube-system
Labels:              <none>
Annotations:         <none>
Image pull secrets:  <none>
Mountable secrets:   <none>
Tokens:              ls-account-secret
Events:              <none>
```

5、 跳转到用户目录`~`，创建 `gen_kubeconfig.sh`，文件内容如下：

```shell
cd ~ # 跳转到用户目录或者拥有.kube的目录
# 创建gen_kubeconfig.sh，将以下内容填入:

#!/bin/bash

NAMESPACE="kube-system"                                           # 命名空间
UserName=pixiu_user                                               # 你的用户名，可以改为任意内容
ApiServerEndpoints=`awk '$0~/server/{print $NF}' ~/.kube/config`  # 从.kube/config获取集群IP
ClusterName=kubernetes                                            # 自定义集群标志，可以改为任意内容
mkdir -p /etc/kubernetes/                                         # kubeconfig存储目录
cd /etc/kubernetes/
SECRET_NAME=ls-account-secret                                     # 上面创建的Secret的名称
# 获取 Secret 中的token
TOKEN=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.token}'| base64 --decode)
CA_CRT=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.ca\.crt}')

# 创建kubeconfig
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
    namespace: ${NAMESPACE}
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
```

6、 执行 `gen_kubeconfig.sh` 生成 `kubeconfig` 文件

```shell
sh gen_kubeconfig.sh
```

7、 查看 `/etc/kubernetes` 是否存在 `kubeconfig` 文件

```shell
cd /etc/kubernetes && ls

admin.conf  controller-manager.conf  kubeconfig  kubelet.conf  ls-scheduler-extender.yaml  manifests  pki  scheduler.conf
```

[回去继续看-> 部署调度拓展为静态POD第五点](部署ls-scheduler-extender静态POD步骤.md)
