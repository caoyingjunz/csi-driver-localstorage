
# 创建自定kubeconfig

1、在此时我们需要创建一个用户并且创建对应的kube.config,用于给上面的静态POD进行使用。以下为一个shell脚本，可以直接使用，也可以参考修改。

```shell
#!/bin/bash

UserName=youName # 你的用户名，可以改为任意内容
ApiServerEndpoints=`awk '$0~/server/{print $NF}' ~/.kube/config`
ClusterName=pixiu-test # 自定义集群标志，可以改为任意内容
NS=kube-system
mkdir -p /etc/kubernetes/pki/client/${UserName}
cd /etc/kubernetes/pki/client/${UserName} # 跳转到这个目录，执行以下命令，生成对应的kubeconfig文件

# 创建用户证书
openssl genrsa -out ${UserName}.key 2048
openssl req -new -key ${UserName}.key -out ${UserName}.csr -subj "/CN=${UserName}"
openssl x509 -req -in ${UserName}.csr -CA /etc/kubernetes/pki/ca.crt \
-CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out ${UserName}.crt -days 3650

# 创建kubeconfig文件
kubectl config set-cluster ${ClusterName} \
  --embed-certs=true \
  --server=${ApiServerEndpoints} \
  --certificate-authority=/etc/kubernetes/pki/ca.crt \
  --kubeconfig=kubeconfig

kubectl config set-credentials ${UserName} \
  --client-certificate=${UserName}.crt \
  --client-key=${UserName}.key \
  --embed-certs=true \
  --kubeconfig=kubeconfig

kubectl config set-context ${UserName}@${ClusterName} \
  --cluster=${ClusterName} \
  --user=${UserName} \
  --namespace=${NS} \
  --kubeconfig=kubeconfig

kubectl config use-context ${UserName}@${ClusterName} --kubeconfig=kubeconfig

# 创建RBAC规则(把创建的用户绑定已经有了的ClusterRole)
kubectl create clusterrolebinding ${UserName}-binding \
  --clusterrole=ls-external-provisioner-role \
  --user=${UserName}

```

2、在此时我们会在`/etc/kubernetes/pki/client/youName`目录下生成一个`kubeconfig`文件，这个文件就是我们需要的`kubeconfig`文件