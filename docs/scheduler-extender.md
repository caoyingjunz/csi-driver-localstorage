# 安装 LocalStorage Scheduler 扩展

### LocalStorage-scheduler
- 生成 `ls-scheduler.conf`, 并拷贝到 `/etc/kubernetes` 目录
  ```shell
  # 生成 pixiu-ls-scheduler.conf

  # 修改 hack/build-lsconfig.sh 的 API_SERVER 为实际地址，然后执行
  ./hack/build-lsconfig.sh
  ```

- 拷贝 `deploy/ls-scheduler.yaml` 到 `/etc/kubernetes/manifests` 目录

- 验证
  ```shell
  # kubectl  get pod -n kube-system pixiu-ls-scheduler-pixiu01
  NAME                         READY   STATUS    RESTARTS       AGE
  pixiu-ls-scheduler-pixiu01   1/1     Running   15 (81s ago)   2d1h

  root@pixiu01:~# kubectl get svc -n kube-system  pixiu-ls-scheduler
  NAME                 TYPE       CLUSTER-IP       EXTERNAL-IP   PORT(S)          AGE
  pixiu-ls-scheduler   NodePort   10.254.245.253   <none>        8090:30666/TCP   2d1h

  # 服务已启动（建议使用 LB）
  curl 10.254.245.253:8090/version
  v1.0.1
  ```

### Kube-scheduler
- 修改 `deploy/ls-scheduler-config.yaml` 的 `urlPrefix` 为实际 `pixiu-ls-scheduler` 地址， 并拷贝到 `/etc/kubernetes` 目录

- 修改 `kube-scheduler` 默认调度配置文件，添加 `pixiu-scheduler-extender` 关联配置
```yaml
  - command:
      - kube-scheduler
      ...
      - --config=/etc/kubernetes/ls-scheduler-config.yaml
    ...
    volumeMounts:
      - mountPath: /etc/kubernetes/scheduler.conf
        name: kubeconfig
        readOnly: true
      - mountPath: /etc/kubernetes/ls-scheduler-config.yaml
        name: ls-scheduler-config
        readOnly: true
  ...
  volumes:
    - hostPath:
        path: /etc/kubernetes/scheduler.conf
        type: FileOrCreate
      name: kubeconfig
    - hostPath:
        path: /etc/kubernetes/ls-scheduler-config.yaml
        type: FileOrCreate
      name: ls-scheduler-config
```
