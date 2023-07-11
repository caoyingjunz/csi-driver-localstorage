# 1、以静态pod的方式部署ls-scheduler-extender

1、先把 `deploy/scheduler-extender-config.yaml` 复制到 `/etc/kubernetes/` 目录下

```bash
cp deploy/scheduler-extender-config.yaml /etc/kubernetes/scheduler-extender-config.yaml
```

2、找到静态POD存放目录，通常在 `/etc/kubernetes/manifests` 修改其中的 `kube-scheduler.yaml` 文件

```bash
cd /etc/kubernetes/manifests
vim kube-scheduler.yaml
```

3、标注 `add` 部分为需要新增内容，修改完保存,系统会自动重启POD

```yaml
# 映射部分
volumeMounts:
  - mountPath: /etc/kubernetes/scheduler.conf
    name: kubeconfig
    readOnly: true
  - mountPath: /etc/kubernetes/scheduler-extender-config.yaml # add
    name: scheduler-extender-config
    readOnly: true
hostNetwork: true
priorityClassName: system-node-critical
volumes:
  - hostPath:
      path: /etc/kubernetes/scheduler.conf
      type: FileOrCreate
    name: kubeconfig
  - hostPath: # add
      path: /etc/kubernetes/scheduler-extender-config.yaml
      type: FileOrCreate
    name: scheduler-extender-config

# 命令行部分新增
containers:
  - command:
      - kube-scheduler
      - --authentication-kubeconfig=/etc/kubernetes/scheduler.conf
      - --authorization-kubeconfig=/etc/kubernetes/scheduler.conf
      - --bind-address=127.0.0.1
      - --kubeconfig=/etc/kubernetes/scheduler.conf
      - --leader-elect=true
      - --config=/etc/kubernetes/scheduler-extender-config.yaml # add
    image: registry.aliyuncs.com/google_containers/kube-scheduler:v1.26.0
```

4、执行 `hack/gen_kubeconfig.sh` 生成自定义 `kubeconfig` 文件

```bash
sh hack/gen_kubeconfig.sh
```

5、查看 `/etc/kubernetes` 是否存在 `kubeconfig` 文件

```bash
cd /etc/kubernetes && ls

admin.conf  controller-manager.conf  kubeconfig  kubelet.conf  ls-scheduler-extender.yaml  manifests  pki  scheduler.conf
```

6、将自定义调度拓展yaml文件 `deploy/ls-scheduler-extender.yaml` 拷贝到 `/etc/kubernetes/manifests/` 目录，POD会自动创建

```bash
kubectl get pods -A

NAMESPACE      NAME                                             READY   STATUS      RESTARTS
kube-system    ls-scheduler-extender-5d678b877b-crcqz           1/1     Running     0             1m
```

7、验证调度拓展是否生效，访问拓展对应的 `service` 的 `version` 接口，如果返回版本数据，则说明调度拓展生效

```bash
kubect get svc -n kube-system # 获取service

NAME                         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                  AGE
kube-dns                     ClusterIP   10.96.0.10      <none>        53/UDP,53/TCP,9153/TCP   109d
ls-scheduler-extender        ClusterIP   10.102.203.92   <none>        8090/TCP                 8d
pixiu-localstorage-service   ClusterIP   10.101.92.72    <none>        443/TCP                  8d

curl 10.102.203.92:8090/version # 访问version接口验证
```
