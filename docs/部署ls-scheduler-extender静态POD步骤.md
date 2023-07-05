# 1、部署ls-scheduler-extender静态POD步骤
部署ls-scheduler-extender静态POD步骤：

1、找到静态POD存放目录，通常在/etc/kubernetes/manifests 这个目录中

2、把deploy/scheduler-extender-config.yaml 复制到/etc/kubernetes/scheduler-extender-config.yaml

3、修改Static Scheduler POD（静态POD） /etc/kubernetes/manifests/kube-scheduler.yaml，修改完保存，系统会自动重启POD ，具体操作如下（修改三处）：

```yaml
cd /etc/kubernetes/manifests/
vim kube-scheduler.yaml


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
4、获取证书和Token为后续的自定义调度静态POD提供权限，并将crt和token的位置修改为指定位置，具体查看:
    [生成crt和token](生成crt和token.md)

5、最后将ls-scheduler-extender.yaml复制到/etc/kubernetes/manifests/目录，POD会自动运行；

```yaml
kubectl get pods -A
NAMESPACE      NAME                                             READY   STATUS      RESTARTS  

kube-system    ls-scheduler-extender-5d678b877b-crcqz           1/1     Running     0             1m
```
