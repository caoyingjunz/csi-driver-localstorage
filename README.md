# Kubez-autoscaler Overview

`kubez-autoscaler` 通过为 `deployment` / `statefulset` 添加 `annotations` 的方式，自动维护对应 `HorizontalPodAutoscaler` 的生命周期.

### Prerequisites

在 `kubernetes` 集群中， 需要先完成 `Metrics Server` 组件的安装，请参考 [Metrics Server](https://github.com/kubernetes-incubator/metrics-server)

`kubectl top node/pod` 验证 `Metrics Server` 已成功安装

``` bash
# kubectl top node
NAME          CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%
kubez         333m         16%    1225Mi          65%

# kubectl top pod
NAME                     CPU(cores)   MEMORY(bytes)
test1-54cd855b77-q67h6   1m           3Mi
```

### Installing

`kubez-autoscaler` 控制器的安装非常简单，通过 `kubectl` 执行 `apply` 如下文件即可完成安装，真正做到猩猩都能使用.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubez
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubez
rules:
  - apiGroups:
      - "*"
    resources:
      - horizontalpodautoscalers
      - deployments
      - statefulsets
      - endpoints
      - leases
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubez
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kubez
subjects:
- kind: ServiceAccount
  name: kubez
  namespace: default
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    kubez.hpa.controller: kubez-autoscaler
  name: kubez-autoscaler-controller
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      kubez.hpa.controller: kubez-autoscaler
  template:
    metadata:
      labels:
        kubez.hpa.controller: kubez-autoscaler
    spec:
      containers:
        - image: jacky06/kubez-autoscaler-controller:v0.0.1
          command:
            - kubez-autoscaler-controller
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
          resources:
            requests:
              cpu: 100m
              memory: 90Mi
          name: kubez-autoscaler-controller
      serviceAccountName: kubez
```

然后通过 `kubectl get pod  -l kubez.hpa.controller=kubez-autoscaler` 能看到 `kubez-autoscaler` 已经启动成功.
```bash
NAME                                          READY   STATUS    RESTARTS   AGE
kubez-autoscaler-controller-dbc4bc4d8-hwpqz   1/1     Running   0          20s
kubez-autoscaler-controller-dbc4bc4d8-tqxrl   1/1     Running   0          20s
```

### Getting Started

在 `deployment` / `statefulset` 的 `annotations` 中添加所需注释即可自动创建对应的 `HPA`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ...
    # 可选，默认 minReplicas 为 1
    hpa.caoyingjunz.io/minReplicas: "2"  # MINPODS
    # 必填
    hpa.caoyingjunz.io/maxReplicas: "6"  # MAXPODS
    ...
    # 支持多种 TARGETS 类型，若开启，至少选择一种，可同时支持多个 TARGETS
    # CPU, in cores. (500m = .5 cores)
    # Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)

    # 使用率 examples
    cpu.hpa.caoyingjunz.io/targetAverageUtilization: "80"
    memory.hpa.caoyingjunz.io/targetAverageUtilization: "60"

    # 使用值 examples
    cpu.hpa.caoyingjunz.io/targetAverageValue: 600m
    memory.hpa.caoyingjunz.io/targetAverageValue: 60Mi

    # TODO: prometheus 暂不支持
    ...
  name: test1
  namespace: default
  ...
```

`kubez-autoscaler-controller` 会根据注释的变化，自动同步 `HPA` 的生命周期.

通过 `kubectl get hpa test1` 命令，可以看到 `deployment` / `statefulset` 关联的 `HPA` 被自动创建
```bash
NAME    REFERENCE          TARGETS            MINPODS   MAXPODS   REPLICAS   AGE
test1   Deployment/test1   6% / 60%           1         2         1          5h29m
test2   Deployment/test2   152%/50%, 0%/60%   2         3         3          34m
```
