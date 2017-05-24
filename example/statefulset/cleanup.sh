#!/usr/bin/env bash

kubectl delete service postgres-demo,governing-postgres
kubectl delete secret postgres-demo-admin-auth
kubectl delete statefulset k8sdb-postgres-demo
