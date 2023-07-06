# 1、部署静态pod调度拓展ls-scheduler-extender

步骤如下：

1、先把`deploy/scheduler-extender-config.yaml` 复制到`/etc/kubernetes/`目录下

```bash
cp deploy/scheduler-extender-config.yaml /etc/kubernetes/scheduler-extender-config.yaml
```

2、找到静态POD存放目录，通常在`/etc/kubernetes/manifests`这个目录中kubernetes官方的kube-apiserver.yaml、kube-controller-manager.yaml、kube-scheduler.yaml、etcd.yaml 都在这个目录下，也就是他们都是以静态pod的形式运行的。

```bash
cd /etc/kubernetes/manifests
vim kube-scheduler.yaml
```

3、修改系统调度器的yaml，也就是`kube-scheduler.yaml`修改完保存，系统会自动重启POD ，具体操作如下（修改三处）：

```yaml
# 映射部分
volumeMounts:
  - mountPath: /etc/kubernetes/scheduler.conf
    name: kubeconfig
    readOnly: true
  - mountPath: /etc/kubernetes/scheduler-extender-config.yaml # 这个也是新加的
    name: scheduler-extender-config
    readOnly: true
hostNetwork: true
priorityClassName: system-node-critical
volumes:
  - hostPath:
      path: /etc/kubernetes/scheduler.conf
      type: FileOrCreate
    name: kubeconfig
  - hostPath: # 这个地方新加的
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
      - --config=/etc/kubernetes/scheduler-extender-config.yaml # 这儿是新增的
    image: registry.aliyuncs.com/google_containers/kube-scheduler:v1.26.0
```

4、获取证书和`Token`为后续的自定义调度静态POD提供权限，并将`ca.crt`和`token`
的位置修改为指定位置，具体查看：[生成crt和token](生成crt和token.md)

5、最后将自定义调度拓展的yaml`deploy/ls-scheduler-extender.yaml`复制到`/etc/kubernetes/manifests/`目录，POD会自动运行；

```shell
kubectl get pods -A

NAMESPACE      NAME                                             READY   STATUS      RESTARTS
kube-system    ls-scheduler-extender-5d678b877b-crcqz           1/1     Running     0             1m
```

6、部署完毕
