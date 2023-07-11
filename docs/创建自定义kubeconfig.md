# 通过ServiceAccount创建kubeconfig文件

1、 执行 `hack/gen_kubeconfig.sh` 生成 `kubeconfig` 文件

```shell
sh hack/gen_kubeconfig.sh
```

2、 查看 `/etc/kubernetes` 是否存在 `kubeconfig` 文件

```shell
cd /etc/kubernetes && ls

admin.conf  controller-manager.conf  kubeconfig  kubelet.conf  ls-scheduler-extender.yaml  manifests  pki  scheduler.conf
```
