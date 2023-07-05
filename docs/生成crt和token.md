# 2、生成crt和token
K8s1.24之后ServiceAccount不自动创建Secret

### 1、kubernetes 1.24 之前，直接shell脚本获取，并给放到data目录下

```yaml
#!/bin/bash

# 设置 ServiceAccount 的名称和命名空间
SERVICEACCOUNT_NAME="csi-ls-node-sa"
NAMESPACE="kube-system"

# 获取 ServiceAccount 的 Secret 的名称
SECRET_NAME=$(kubectl get serviceaccount ${SERVICEACCOUNT_NAME} -n ${NAMESPACE} -o jsonpath='{.secrets[0].name}')

# 获取 Secret 中的证书和 token
CA_CRT=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.ca\.crt}')
TOKEN=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.token}')

# 将 base64 编码的证书和 token 解码为文件
echo ${CA_CRT} | base64 --decode > ca.crt
echo ${TOKEN} | base64 --decode > token


#mv ca.crt /data/ca.crt
#mv token /data/token
```
### 2、kubernetes1.24之后，需要手动创建Secret并进行关联，然后再给放到data目录下

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ls-secret-token
  namespace: kube-system
  annotations:
    kubernetes.io/service-account.name: "csi-ls-node-sa"   # 这里填写serviceAccountName
type: kubernetes.io/service-account-token
```
查看ServiceAccount 的信息，看看Token中有没有关联上

```shell
kubectl describe sa  csi-ls-node-sa -n kube-system

Name:                csi-ls-node-sa
Namespace:           kube-system
Labels:              <none>
Annotations:         <none>
Image pull secrets:  <none>
Mountable secrets:   <none>
Tokens:              ls-secret-token
Events:              <none>
```
然后：

```shell
SECRET_NAME = ls-secret-token
NAMESPACE="kube-system"
# 获取 Secret 中的证书和 token
CA_CRT=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.ca\.crt}')
TOKEN=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath='{.data.token}')

# 将 base64 编码的证书和 token 解码为文件
echo ${CA_CRT} | base64 --decode > ca.crt
echo ${TOKEN} | base64 --decode > token
```


最后映射文件到POD中:

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
      volumeMounts:
        - name: kube-data
          mountPath: /var/run/secrets/kubernetes.io/serviceaccount/token
          subPath: token
        - name: kube-data
          mountPath: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
          subPath: ca.crt
  volumes:
    - name: kube-data
      hostPath:
        path: /data

```
[回去继续看-> 部署调度拓展为静态POD](部署ls-scheduler-extender静态POD步骤.md)