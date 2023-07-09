# Kube-scheduler

- 修改 `kube-scheduler` 默认调度配置文件，添加 `pixiu-scheduler-extender` 关联配置
```yaml
  - command:
      - kube-scheduler
      ...
      - --config=/etc/kubernetes/scheduler-extender-config.yaml
    ...
    volumeMounts:
      - mountPath: /etc/kubernetes/scheduler.conf
        name: kubeconfig
        readOnly: true
      - mountPath: /etc/kubernetes/scheduler-extender-config.yaml
        name: scheduler-extender-config
        readOnly: true
  ...
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