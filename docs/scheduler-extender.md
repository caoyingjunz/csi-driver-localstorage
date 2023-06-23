# Scheduler

- 修改默认调度，添加 `scheduler-extender-config` 配置
```yaml
volumeMounts:
  - mountPath: /etc/kubernetes/scheduler.conf
    name: kubeconfig
    readOnly: true
  - mountPath: /etc/kubernetes/scheduler-extender-config.yaml
    name: scheduler-extender-config
    readOnly: true
hostNetwork: true
priorityClassName: system-node-critical
volumes:
  - hostPath:
      path: /etc/kubernetes/scheduler.conf
      type: FileOrCreate
    name: kubeconfig
  - hostPath:
      path: /etc/kubernetes/scheduler-extender-config.yaml
      type: FileOrCreate
    name: scheduler-extender-config
```