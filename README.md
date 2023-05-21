# LocalStorage

![Build Status][build-url]
[![Release][release-image]][release-url]
[![License][license-image]][license-url]

## Overview
This driver allows Kubernetes to access LocalStorage on Linux node.

## Getting Started

### Installation
- 安装 `csi-localstorage` 组件
    ```shell
    kubectl apply -f deploy/v1.0.0

    # 验证
    kubectl get pod -l app=csi-ls-node -n kube-system
    NAME                READY   STATUS    RESTARTS   AGE
    csi-ls-node-kcbvr   3/3     Running   0          13m
    ```

- 安装 `storageclass`
    ```shell
    kubectl apply -f deploy/storageclass.yaml

    # 验证
    kubectl get sc localstorage.caoyingjunz.io
    NAME                          PROVISIONER                       RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
    localstorage.caoyingjunz.io   localstorage.csi.caoyingjunz.io   Delete          WaitForFirstConsumer   false                  8m15s
    ```

### demo
- 创建 pvc
    ```shell
    kubectl apply -f examples/pvc.yaml

    # 验证
    kubectl get pvc
    NAME                STATUS   VOLUME                                       CAPACITY   ACCESS MODES   STORAGECLASS                  AGE
    test-localstorage   Bound    pvc-c9a48b2b-248d-4e8a-917c-4a63c570292b     1Mi        RWX            localstorage.caoyingjunz.io   5s
    ```

## 学习分享
- [go-learning](https://github.com/caoyingjunz/go-learning)

## 沟通交流
- 搜索微信号 `yingjuncz`, 备注（localstorage）, 验证通过会加入群聊
- [bilibili](https://space.bilibili.com/3493104248162809?spm_id_from=333.1007.0.0) 技术分享

Copyright 2019 caoyingjun (cao.yingjunz@gmail.com) Apache License 2.0

[build-url]: https://github.com/caoyingjunz/csi-driver-localstorage/actions/workflows/ci.yml/badge.svg
[release-image]: https://img.shields.io/badge/release-download-orange.svg
[release-url]: https://www.apache.org/licenses/LICENSE-2.0.html
[license-image]: https://img.shields.io/badge/license-Apache%202-4EB1BA.svg
[license-url]: https://www.apache.org/licenses/LICENSE-2.0.html
