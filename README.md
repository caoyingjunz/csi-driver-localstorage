# Pixiu(貔貅) Overview

`Pixiu` 旨在对 `kubernetes` 原生功能的补充和强化

- 提供 `kubernetes` 层面的镜像管理能力
  - 可通过 `kubectl` 或 `client-go` 对集群中的 `images` 进行管理
  ```
  # kubectl get images
  NAME         AGE   IMAGE
  image-test   33h   nginx:1.9.2
  ```
  - 通过注释，在创建 `deployment` 等资源的时候，开启镜像拉取功能，自动在指定 `node` 完成镜像准备

- 无状态应用的分批发布
  ```
  # kubectl get advancedDeployment
  NAME         READY   UP-TO-DATE   AVAILABLE   AGE
  example-ad   3       3            3           4d2h
  ```

### Installing (demo版)

`pixiu` 安装非常简单，通过 `kubectl` 执行 `apply` 如下文件即可完成安装，真正做到猩猩都能使用.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: pixiu-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pixiu
  namespace: pixiu-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pixiu
rules:
  - apiGroups:
      - apps.pixiu.io
    resources:
      - advanceddeployment
      - imagesets
    verbs:
      - get
      - list
      - patch
      - update
      - watch
  - apiGroups:
      - "*"
    resources:
      - endpoints
      - leases
    verbs:
      - get
      - list
      - patch
      - update
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: pixiu
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: pixiu
subjects:
- kind: ServiceAccount
  name: pixiu
  namespace: default
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pixiu-controller-manager
  namespace: pixiu-system
spec:
  replicas: 1
  selector:
    matchLabels:
      pixiu.controller.manager: pixiu-controller-manager
  template:
    metadata:
      labels:
        pixiu.controller.manager: pixiu-controller-manager
    spec:
      containers:
        - image: jacky06/pixiu-controller-manager:v0.0.1
          command:
            - pixiu-controller-manager
            - --leader-elect=true
          imagePullPolicy: IfNotPresent
          livenessProbe:
            failureThreshold: 8
            httpGet:
              host: 127.0.0.1
              path: /healthz
              port: 10256
              scheme: HTTP
            initialDelaySeconds: 15
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 15
          name: pixiu-controller-manager
      serviceAccountName: pixiu
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: pixiu-daemon
  namespace: pixiu-system
spec:
  selector:
    matchLabels:
      pixiu.daemon: pixiu-daemon
  template:
    metadata:
      labels:
        pixiu.daemon: pixiu-daemon
    spec:
      serviceAccountName: pixiu
      containers:
      - command:
        - pixiu-daemon
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: jacky06/pixiu-daemon:v0.0.1
        imagePullPolicy: IfNotPresent
        name: pixiu-daemon
        volumeMounts:
        - mountPath: /var/run
          name: socketpath
      hostNetwork: true
      volumes:
      - hostPath:
          path: /var/run
          type: ""
        name: socketpath
```
