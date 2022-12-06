---
title: Kubernetes Pod CrashLooping
weight: 20
---

# KubernetesPodCrashLooping

## Meaning

Kubernetes Pods are not in `CrashLoopBackOff` status.

## Impact

The applications backed by the unhealthy pods might be down or degraded.

## Diagnosis

```bash
kubectl get pod <crashed pod name> -n <crashed pod namespace>

NAMESPACE   NAME                    READY   STATUS              RESTARTS    AGE
default     alertmanager-main-0     1/2     CrashLoopBackOff             37107 2d
default     alertmanager-main-1     2/2     Running             0 43d
default     alertmanager-main-2     2/2     Running             0 43d 
```

Find the root cause by looking to events for a given pod/deployement

```
kubectl get events --field-selector involvedObject.name=alertmanager-main-0
```

## Mitigation

- Make sure pods have enough resources (CPU, MEM) to work correctly.
- Check the unhealthy pods' logs, which might have failed volume/secret/configMap mount, errors in the code, waiting for other dependencies, etc.
```
kubectl logs alertmanager-main-0 -n -f
```