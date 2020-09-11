# Lokomotive admission webhooks

As part of the cluster control-plane Lokomotive creates additional admission webhooks, which provides extra features to the cluster. This document describes what webhooks we install and what they do.

## Default ServiceAccount Mutating Webhook

When you create a pod, if you do not specify a service account, it is automatically assigned the `default` service account in the same namespace. If you get the raw JSON or YAML for a pod you have created (for example, `kubectl get pods/<podname> -o yaml`), you can see the `spec.serviceAccountName` field has been automatically set. For more information see [kubernetes docs](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/). By default, created pods can authenticate to Kubernetes API, which might be a potential security threat.

Not every pod needs the ability to utilize the API from within itself. If your application do not integrate with Kubernetes and not utilize it's API, it shouldn't have credentials for it.

To avoid manually disabling automounting of default service account, we have a webhook server that patches default service accounts whenever you either apply any [lokomotive component](./components.md) or create a new namespace. To see how it works follow below steps from your command line.

```bash
# Create a namespace.
$ kubectl create ns foo

# Get default service for namespace foo.
$ kubectl get sa default -o yaml -n foo
```

After following the above steps you can see that `automountServiceAccountToken` field is set to false.

### Current limitations

Currently, on applying cluster `default` service account for `default` namespace is not patched. Please note, that as general convention, `default` namespace should not be used to run your workloads. However, if you want it patched too, you can delete the default service account after your cluster is created.

`$ kubectl delete sa default -n default`

### How to enable mounting default service account for pods

It is recommended to create a dedicated service account for your application, so that you have full control over grants it has bind. However, if you still want to enable mounting for default service account, you can opt in of automounting API credentials for a particular pod by setting `automountServiceAccountToken: true`. The pod spec takes precedence over the service account if both specify a value for `automountServiceAccountToken` field.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  serviceAccountName: build-robot
  automountServiceAccountToken: true
  ...
```
