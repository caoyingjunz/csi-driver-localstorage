# 2、生成ca.crt和token

前置知识：
> 生成ca.crt和token 主要是为了后续的自定义调度静态POD提供权限，经过查阅文献，在kubernetes1.24之前，创建ServiceAccount会自动生成对应的Secret，在Secret中会存储对应的crt和token。而在kubernetes1.24之后，创建ServiceAccount不会自动创建Secret，需要手动创建Secret，并从其中获取ca.crt和token。

**所以我们需要从`ServiceAccount`对应的Secret中去获取`ca.crt`和`token`，本文根据kubernetes1.24前和1.24后，提供了两种解决方案，具体步骤如下**：

1、首先需要先检查一下是否部署`csi-ls-node-sa`这个`ServiceAccount`，如果没有需要先创建，如果有则跳过此步骤。

```shell
# 查看是否部署csi-ls-node-sa这个ServiceAccount
kubectl get sa -n kube-system

NAME                                 SECRETS   AGE
csi-ls-node-sa                       1         1m

# 没有的话创建(其中的v1.0.1为每个版本的版本号，根据最新版本进行调整)
kubectl apply -f deploy/v1.0.1/ls-rbac.yaml
```

2、查看kubernetes 版本，判断版本号是否大于1.24，具体操作如下：

```shell
# 查看kubernetes 版本
kubectl version
```

3、如果kubernetes版本小于1.24，直接复制以下shell命令执行，会自动创建`get_token.sh`
这个文件，在创建文件完成之后，会自动执行`sh get_token.sh`生成对应的`ca.crt`和`token`并将文件自动移动至`/data`目录下。

```shell
cat << EOF > get_token.sh
#!/bin/bash

# 设置 ServiceAccount 的名称和命名空间
SERVICEACCOUNT_NAME="csi-ls-node-sa"
NAMESPACE="kube-system"

# 获取 ServiceAccount 的 Secret 的名称
SECRET_NAME=\$(kubectl get serviceaccount \${SERVICEACCOUNT_NAME} -n \${NAMESPACE} -o jsonpath='{.secrets[0].name}')

# 获取 `Secret` 中的证书和 token
CA_CRT=\$(kubectl get secret \${SECRET_NAME} -n \${NAMESPACE} -o jsonpath='{.data.ca\\.crt}')
TOKEN=\$(kubectl get secret \${SECRET_NAME} -n \${NAMESPACE} -o jsonpath='{.data.token}')

# 将 base64 编码的证书和 token 解码为文件
echo \${CA_CRT} | base64 --decode > ca.crt
echo \${TOKEN} | base64 --decode > token

mv ca.crt /data/ca.crt
mv token /data/token
EOF

chmod +x get_token.sh
sh get_token.sh
```

4、如果kubernetes版本大于`1.24`，则需要手动创建Secret并进行关联，然后再从Secret中获取对应的`ca.crt`和`token`，具体步骤如下：

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

4.1 部署Secret:

```shell
kubectl apply -f ls-account-secret.yaml
```

4.2 查看Secret是否创建成功:

```shell
kubectl get secret ls-account-secret -n kube-system
```

4.3 查看`ServiceAccount` 的信息，看看`Token`中有没有关联上上面创建的`Secret`:

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

4.4 直接复制以下shell命令执行，会自动创建`get_token.sh`这个文件，在创建文件完成之后，会自动执行`sh get_token.sh`
生成对应的`ca.crt`和`token`，并将文件自动移动至`/data`目录下。

```shell
cat << EOF > get_token.sh
#!/bin/bash
# 设置 ServiceAccount 的名称和命名空间
SECRET_NAME = ls-account-secret # 这里填写上面创建的Secret的名称
NAMESPACE="kube-system"
# 获取 Secret 中的证书和 token
CA_CRT=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.ca\.crt}')
TOKEN=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.token}')

# 将 base64 编码的证书和 token 解码为文件
echo ${CA_CRT} | base64 --decode > ca.crt
echo ${TOKEN} | base64 --decode > token

mv ca.crt /data/ca.crt
mv token /data/token
EOF

chmod +x get_token.sh
sh get_token.sh
```

4.5 因为`ca.crt`和`token`是被移动到data目录的，所以将data目录通过`hostPath`映射到POD中，并在`volumeMounts`中指定挂载的路径，如下:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ls-scheduler-extender
  namespace: kube-system
  labels:
    app: ls-scheduler-extender
spec:
  containers:
    - args:
        - -v=2
        - --port=8090
      image: harbor.powerlaw.club/pixiuio/scheduler-extender:latest
      imagePullPolicy: Always
      name: ls-scheduler-extender
      volumeMounts: # 挂载data目录
        - name: kube-data
          mountPath: /var/run/secrets/kubernetes.io/serviceaccount/token
          subPath: token
        - name: kube-data
          mountPath: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
          subPath: ca.crt
  volumes: # 指定挂载的路径
    - name: kube-data
      hostPath:
        path: /data

```

[回去继续看-> 部署调度拓展为静态POD第五点](部署ls-scheduler-extender静态POD步骤.md)
