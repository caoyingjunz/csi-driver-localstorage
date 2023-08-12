# LocalStorage

![Build Status][build-url]
[![Release][release-image]][release-url]
[![License][license-image]][license-url]

## Overview
This driver allows Kubernetes to access LocalStorage on Linux node.

## Getting Started

### Installation
- 选择运行 `localstorage` 的 `kubernetes` 节点
  ```shell
  kubectl label node <node-name> storage.caoyingjunz.io/node=
  ```

- 安装 `localstorage` 组件
  ```shell
  kubectl apply -f deploy/crds
  kubectl apply -f deploy/latest

  # 验证
  root@pixiu01:~# kubectl get pod -n kube-system | grep pixiu
  pixiu-ls-controller-7997b8c446-8hndc        1/1     Running            0                87s
  pixiu-ls-node-gfd78                         4/4     Running            0                87s
  pixiu-ls-node-mb4x6                         4/4     Running            0                87s
  ```

- 安装 [LocalStorage 调度扩展](./docs/scheduler-extender.md)

- 创建 `localstorage` 资源
  ```shell
  # 自动创建
  kubectl apply -f deploy/ls-job.yaml

  # 手动创建
  # 修改 examples/ls.yaml, 多个 node 节点需要多次创建
  kubectl apply -f examples/ls.yaml

  # 验证
  kubectl get ls
  NAME         STATUS   KUBENODE   ALLOCATABLE   CAPACITY   AGE
  ls-pixiu01   Ready    pixiu01    360Gi         360Gi      16d
  ```

- 安装 `storageclass`
  ```shell
  kubectl apply -f deploy/ls-storageclass.yaml

  # 验证
  kubectl get sc pixiu-localstorage
  NAME                 PROVISIONER                       RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
  pixiu-localstorage   localstorage.csi.caoyingjunz.io   Delete          WaitForFirstConsumer   false                  2m54s
  ```

- 创建 `pvc` 验证
  ```shell
  kubectl apply -f examples/pvc.yaml

  # 验证
  kubectl get pvc
  NAME                 STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS              AGE
  test-pvc             Bound    pvc-2b2c809f-33b5-437f-a4b8-61906c10a3e1   1Mi        RWX            pixiu-localstorage        5s
  ```

## Feature
- Schedule with volume status
- Volume metrics

## 学习分享
- [go-learning](https://github.com/caoyingjunz/go-learning)

## 沟通交流
- 搜索微信号 `yingjuncz`, 备注（ls）, 验证通过会加入群聊
- [bilibili](https://space.bilibili.com/3493104248162809?spm_id_from=333.1007.0.0) 技术分享

Copyright 2019 caoyingjun (cao.yingjunz@gmail.com) Apache License 2.0

[build-url]: https://github.com/caoyingjunz/csi-driver-localstorage/actions/workflows/ci.yml/badge.svg
[release-image]: https://img.shields.io/badge/release-download-orange.svg
[release-url]: https://www.apache.org/licenses/LICENSE-2.0.html
[license-image]: https://img.shields.io/badge/license-Apache%202-4EB1BA.svg
[license-url]: https://www.apache.org/licenses/LICENSE-2.0.html
