#!/usr/bin/env bash

kubectl delete sa governing-postgres
kubectl delete service postgres-demo
kubectl delete secret postgres-demo-admin-auth
kubectl delete statefulset db-postgres-demo
