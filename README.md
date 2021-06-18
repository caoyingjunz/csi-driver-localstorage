# Pixiu(貔貅) Overview

`Pixiu` 旨在对 `kubernetes` 原生功能的补充和强化

- 提供 `kubernetes` 层面的镜像管理能力
  - 可通过 `kubectl` 或 `client-go` 对集群中的 `images` 进行管理
  ```
  # kubectl get images
  NAME         AGE   IMAGE
  image-test   33h   nginx:1.9.2
  ```
  - 通过注释，在创建 `deployment` 等资源的时候，开启镜像拉取功能，自动在指定 `node` 完成镜像准备

- 无状态应用的分批发布
  ```
  # kubectl get advancedDeployment
  NAME         READY   UP-TO-DATE   AVAILABLE   AGE
  example-ad   3       3            3           4d2h
  ```
