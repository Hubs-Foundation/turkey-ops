---
title: Kubernetes Deployment Has Mismatched Replicas
weight: 20
---

# KubernetesDeploymentReplicasMismatch

## Meaning

Kubernetes Deployment has unexpected replicas

## Impact

The applications backed by this deployment might be down or degraded.

## Diagnosis

```bash
kubectl get deploy <mismatched deployment name> -n <mismatched deployment namespace>

NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
alertmanager         0/1     1            0           49d
grafana              1/1     1            1           49d
kube-state-metrics   1/1     1            1           49d
prometheus           1/1     1            1           49d
```

Find the root cause by looking to events for a given pod/deployement

```
kubectl get events --field-selector involvedObject.name=alertmanager
```

## Mitigation

- Make sure pods have enough resources (CPU, MEM) to work correctly.
- Check if the pods are using the correct nodeSelector.
```
kubectl get pod <PENDING_POD_NAME> -ojson | jq '.spec.nodeSelector
```
- Check the unhealthy pods' logs, which might have failed volume/secret/configMap mount, errors in the code, waiting for other dependencies, etc.
```
kubectl logs alertmanager-main-0 -n -f
```