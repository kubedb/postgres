# Change Log

## [0.8.0-rc.0](https://github.com/kubedb/postgres/tree/0.8.0-rc.0) (2018-05-28)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.8.0-beta.2...0.8.0-rc.0)

**Merged pull requests:**

- Update release script [\#143](https://github.com/kubedb/postgres/pull/143) ([tamalsaha](https://github.com/tamalsaha))
- Fixed kubeconfig plugin for Cloud Providers && Storage is required for Postgres [\#142](https://github.com/kubedb/postgres/pull/142) ([the-redback](https://github.com/the-redback))
-  Concourse [\#141](https://github.com/kubedb/postgres/pull/141) ([tahsinrahman](https://github.com/tahsinrahman))
-  Refactored E2E testing to support self-hosted operator with proper deployment configuration [\#140](https://github.com/kubedb/postgres/pull/140) ([the-redback](https://github.com/the-redback))
- Skip delete requests for empty resources [\#139](https://github.com/kubedb/postgres/pull/139) ([the-redback](https://github.com/the-redback))
- Don't panic if admission options is nil [\#138](https://github.com/kubedb/postgres/pull/138) ([tamalsaha](https://github.com/tamalsaha))
- Disable admission controllers for webhook server [\#137](https://github.com/kubedb/postgres/pull/137) ([tamalsaha](https://github.com/tamalsaha))
- Separate ApiGroup for Mutating and Validating webhook && upgraded osm to 0.7.0 [\#136](https://github.com/kubedb/postgres/pull/136) ([the-redback](https://github.com/the-redback))
- Update client-go to 7.0.0 [\#135](https://github.com/kubedb/postgres/pull/135) ([tamalsaha](https://github.com/tamalsaha))
- Bundle Webhook Server and Added sharedinfomer Factory [\#132](https://github.com/kubedb/postgres/pull/132) ([the-redback](https://github.com/the-redback))
-  Moved ValidatingWebhook Packages from kubedb-server to postgres repo [\#131](https://github.com/kubedb/postgres/pull/131) ([the-redback](https://github.com/the-redback))
- Add travis yaml [\#130](https://github.com/kubedb/postgres/pull/130) ([tahsinrahman](https://github.com/tahsinrahman))

## [0.8.0-beta.2](https://github.com/kubedb/postgres/tree/0.8.0-beta.2) (2018-02-27)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.8.0-beta.1...0.8.0-beta.2)

**Implemented enhancements:**

- use separate script for different task [\#126](https://github.com/kubedb/postgres/pull/126) ([aerokite](https://github.com/aerokite))

**Fixed bugs:**

- use separate script for different task [\#126](https://github.com/kubedb/postgres/pull/126) ([aerokite](https://github.com/aerokite))

**Merged pull requests:**

- Use apps/v1 [\#128](https://github.com/kubedb/postgres/pull/128) ([aerokite](https://github.com/aerokite))
- upgrade version & fixed service [\#127](https://github.com/kubedb/postgres/pull/127) ([aerokite](https://github.com/aerokite))
- Fix for pointer type [\#125](https://github.com/kubedb/postgres/pull/125) ([aerokite](https://github.com/aerokite))
- Fix dormantDB matching: pass same type to Equal method [\#124](https://github.com/kubedb/postgres/pull/124) ([the-redback](https://github.com/the-redback))
- Add support of Postgres 10.2 [\#123](https://github.com/kubedb/postgres/pull/123) ([aerokite](https://github.com/aerokite))
- Fixed dormantdb matching & Raised throttling time & Fixed Postgres version checking [\#121](https://github.com/kubedb/postgres/pull/121) ([the-redback](https://github.com/the-redback))
- Use official code generator scripts [\#120](https://github.com/kubedb/postgres/pull/120) ([tamalsaha](https://github.com/tamalsaha))
- Fix merge service ports [\#119](https://github.com/kubedb/postgres/pull/119) ([aerokite](https://github.com/aerokite))

## [0.8.0-beta.1](https://github.com/kubedb/postgres/tree/0.8.0-beta.1) (2018-01-29)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.8.0-beta.0...0.8.0-beta.1)

**Merged pull requests:**

- Reorg docker code structure [\#117](https://github.com/kubedb/postgres/pull/117) ([aerokite](https://github.com/aerokite))

## [0.8.0-beta.0](https://github.com/kubedb/postgres/tree/0.8.0-beta.0) (2018-01-07)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.7.1...0.8.0-beta.0)

**Merged pull requests:**

- Update rbac role [\#116](https://github.com/kubedb/postgres/pull/116) ([aerokite](https://github.com/aerokite))
- update docker image validation [\#115](https://github.com/kubedb/postgres/pull/115) ([aerokite](https://github.com/aerokite))
- Use work queue [\#114](https://github.com/kubedb/postgres/pull/114) ([aerokite](https://github.com/aerokite))
- Reorg location of docker images [\#113](https://github.com/kubedb/postgres/pull/113) ([aerokite](https://github.com/aerokite))
- Set client id for analytics [\#112](https://github.com/kubedb/postgres/pull/112) ([tamalsaha](https://github.com/tamalsaha))
- delete configmap used for leader-election [\#110](https://github.com/kubedb/postgres/pull/110) ([aerokite](https://github.com/aerokite))
- Various fixes in docker & controller [\#109](https://github.com/kubedb/postgres/pull/109) ([aerokite](https://github.com/aerokite))
- Update to use Archiver pointer [\#108](https://github.com/kubedb/postgres/pull/108) ([aerokite](https://github.com/aerokite))
- Fix CRD registration [\#107](https://github.com/kubedb/postgres/pull/107) ([the-redback](https://github.com/the-redback))
- Added log-based archive support with wal-g in postgres [\#106](https://github.com/kubedb/postgres/pull/106) ([aerokite](https://github.com/aerokite))
- Remove dependency on deleted appscode/log packages. [\#105](https://github.com/kubedb/postgres/pull/105) ([tamalsaha](https://github.com/tamalsaha))
- Use monitoring tools from appscode/kutil [\#104](https://github.com/kubedb/postgres/pull/104) ([tamalsaha](https://github.com/tamalsaha))
- fixes k8sdb/operator\#126 for postgres part [\#103](https://github.com/kubedb/postgres/pull/103) ([the-redback](https://github.com/the-redback))
- Use client-go 5.x [\#102](https://github.com/kubedb/postgres/pull/102) ([tamalsaha](https://github.com/tamalsaha))
- Update secret procedure for Restore [\#101](https://github.com/kubedb/postgres/pull/101) ([the-redback](https://github.com/the-redback))

## [0.7.1](https://github.com/kubedb/postgres/tree/0.7.1) (2017-10-04)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.7.0...0.7.1)

## [0.7.0](https://github.com/kubedb/postgres/tree/0.7.0) (2017-09-26)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.6.0...0.7.0)

**Merged pull requests:**

- Assign Kind Type in CRD object [\#100](https://github.com/kubedb/postgres/pull/100) ([aerokite](https://github.com/aerokite))
- Set Affinity and Tolerations from CRD spec [\#99](https://github.com/kubedb/postgres/pull/99) ([tamalsaha](https://github.com/tamalsaha))
- Support migration from TPR to CRD [\#98](https://github.com/kubedb/postgres/pull/98) ([aerokite](https://github.com/aerokite))
- Use kutil in e2e-test [\#97](https://github.com/kubedb/postgres/pull/97) ([aerokite](https://github.com/aerokite))
- Resume DormantDatabase while creating Original DB again [\#96](https://github.com/kubedb/postgres/pull/96) ([aerokite](https://github.com/aerokite))
- Rewrite e2e tests using ginkgo [\#95](https://github.com/kubedb/postgres/pull/95) ([aerokite](https://github.com/aerokite))

## [0.6.0](https://github.com/kubedb/postgres/tree/0.6.0) (2017-07-24)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.5.0...0.6.0)

**Merged pull requests:**

- Revendor for api fix [\#94](https://github.com/kubedb/postgres/pull/94) ([aerokite](https://github.com/aerokite))

## [0.5.0](https://github.com/kubedb/postgres/tree/0.5.0) (2017-07-19)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.4.0...0.5.0)

## [0.4.0](https://github.com/kubedb/postgres/tree/0.4.0) (2017-07-18)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.3.1...0.4.0)

## [0.3.1](https://github.com/kubedb/postgres/tree/0.3.1) (2017-07-14)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.3.0...0.3.1)

## [0.3.0](https://github.com/kubedb/postgres/tree/0.3.0) (2017-07-08)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.2.0...0.3.0)

**Merged pull requests:**

- e2e test for backup in local directory [\#93](https://github.com/kubedb/postgres/pull/93) ([aerokite](https://github.com/aerokite))
- Support RBAC [\#92](https://github.com/kubedb/postgres/pull/92) ([aerokite](https://github.com/aerokite))
- Allow setting resources for StatefulSet or Snapshot/Restore jobs [\#91](https://github.com/kubedb/postgres/pull/91) ([tamalsaha](https://github.com/tamalsaha))
- Use updated snapshot storage format [\#90](https://github.com/kubedb/postgres/pull/90) ([tamalsaha](https://github.com/tamalsaha))
- Add app=kubedb labels to TPR reg [\#89](https://github.com/kubedb/postgres/pull/89) ([tamalsaha](https://github.com/tamalsaha))
- Support using non-default service account [\#88](https://github.com/kubedb/postgres/pull/88) ([tamalsaha](https://github.com/tamalsaha))
- Separate validation [\#87](https://github.com/kubedb/postgres/pull/87) ([aerokite](https://github.com/aerokite))

## [0.2.0](https://github.com/kubedb/postgres/tree/0.2.0) (2017-06-22)
[Full Changelog](https://github.com/kubedb/postgres/compare/0.1.0...0.2.0)

**Merged pull requests:**

- Expose exporter port via service [\#86](https://github.com/kubedb/postgres/pull/86) ([tamalsaha](https://github.com/tamalsaha))
- Correctly parse target port [\#85](https://github.com/kubedb/postgres/pull/85) ([tamalsaha](https://github.com/tamalsaha))
- Run side car exporter [\#84](https://github.com/kubedb/postgres/pull/84) ([tamalsaha](https://github.com/tamalsaha))
- get summary report [\#83](https://github.com/kubedb/postgres/pull/83) ([aerokite](https://github.com/aerokite))
- Use client-go [\#82](https://github.com/kubedb/postgres/pull/82) ([tamalsaha](https://github.com/tamalsaha))

## [0.1.0](https://github.com/kubedb/postgres/tree/0.1.0) (2017-06-14)
**Fixed bugs:**

- Allow updating to create missing workloads [\#78](https://github.com/kubedb/postgres/pull/78) ([aerokite](https://github.com/aerokite))

**Merged pull requests:**

- Change api version to v1alpha1 [\#81](https://github.com/kubedb/postgres/pull/81) ([tamalsaha](https://github.com/tamalsaha))
- Pass cronController as parameter [\#80](https://github.com/kubedb/postgres/pull/80) ([aerokite](https://github.com/aerokite))
- Use built-in exporter [\#79](https://github.com/kubedb/postgres/pull/79) ([tamalsaha](https://github.com/tamalsaha))
- Add analytics event for operator [\#77](https://github.com/kubedb/postgres/pull/77) ([aerokite](https://github.com/aerokite))
- Add analytics [\#76](https://github.com/kubedb/postgres/pull/76) ([aerokite](https://github.com/aerokite))
- Use util tag matching TPR version [\#75](https://github.com/kubedb/postgres/pull/75) ([tamalsaha](https://github.com/tamalsaha))
- Revendor client-go [\#74](https://github.com/kubedb/postgres/pull/74) ([tamalsaha](https://github.com/tamalsaha))
- Add Run\(\) method to just run controller. [\#73](https://github.com/kubedb/postgres/pull/73) ([tamalsaha](https://github.com/tamalsaha))
- Add HTTP server to expose metrics [\#72](https://github.com/kubedb/postgres/pull/72) ([tamalsaha](https://github.com/tamalsaha))
- Prometheus support [\#71](https://github.com/kubedb/postgres/pull/71) ([saumanbiswas](https://github.com/saumanbiswas))
- Use kubedb docker hub account [\#70](https://github.com/kubedb/postgres/pull/70) ([tamalsaha](https://github.com/tamalsaha))
- Rename operator name [\#69](https://github.com/kubedb/postgres/pull/69) ([aerokite](https://github.com/aerokite))
- Use kubedb.com apigroup instead of k8sdb.com [\#68](https://github.com/kubedb/postgres/pull/68) ([tamalsaha](https://github.com/tamalsaha))
- Do not handle DormantDatabase [\#67](https://github.com/kubedb/postgres/pull/67) ([aerokite](https://github.com/aerokite))
- Pass clients instead of config [\#66](https://github.com/kubedb/postgres/pull/66) ([aerokite](https://github.com/aerokite))
- Ungroup imports on fmt [\#65](https://github.com/kubedb/postgres/pull/65) ([tamalsaha](https://github.com/tamalsaha))
- Fix go report card issues [\#64](https://github.com/kubedb/postgres/pull/64) ([tamalsaha](https://github.com/tamalsaha))
- Use common receiver [\#63](https://github.com/kubedb/postgres/pull/63) ([tamalsaha](https://github.com/tamalsaha))
- Rename delete database to pause [\#62](https://github.com/kubedb/postgres/pull/62) ([tamalsaha](https://github.com/tamalsaha))
- Rename DeletedDatabase to DormantDatabase [\#61](https://github.com/kubedb/postgres/pull/61) ([tamalsaha](https://github.com/tamalsaha))
- Add e2e test for updating scheduler [\#60](https://github.com/kubedb/postgres/pull/60) ([aerokite](https://github.com/aerokite))
- Fix update method [\#59](https://github.com/kubedb/postgres/pull/59) ([aerokite](https://github.com/aerokite))
- Remove prefix from snapshot job [\#58](https://github.com/kubedb/postgres/pull/58) ([aerokite](https://github.com/aerokite))
- Delete Database Secret for wipe out [\#57](https://github.com/kubedb/postgres/pull/57) ([aerokite](https://github.com/aerokite))
- Rename DatabaseSnapshot to Snapshot [\#56](https://github.com/kubedb/postgres/pull/56) ([tamalsaha](https://github.com/tamalsaha))
- Modify StatefulSet naming format [\#54](https://github.com/kubedb/postgres/pull/54) ([aerokite](https://github.com/aerokite))
- Get object each time before updating [\#53](https://github.com/kubedb/postgres/pull/53) ([aerokite](https://github.com/aerokite))
- Check docker image version [\#52](https://github.com/kubedb/postgres/pull/52) ([aerokite](https://github.com/aerokite))
- Create headless service for StatefulSet [\#51](https://github.com/kubedb/postgres/pull/51) ([aerokite](https://github.com/aerokite))
- Use data as Volume name [\#50](https://github.com/kubedb/postgres/pull/50) ([aerokite](https://github.com/aerokite))
- Put kind in label instead of type [\#48](https://github.com/kubedb/postgres/pull/48) ([aerokite](https://github.com/aerokite))
- Do not store autogenerated meta information [\#47](https://github.com/kubedb/postgres/pull/47) ([aerokite](https://github.com/aerokite))
- Bubble up error for controller methods [\#45](https://github.com/kubedb/postgres/pull/45) ([aerokite](https://github.com/aerokite))
- Modify e2e test. Do not support recovery by recreating Postgres anymore [\#44](https://github.com/kubedb/postgres/pull/44) ([aerokite](https://github.com/aerokite))
- Use Kubernetes EventRecorder directly [\#43](https://github.com/kubedb/postgres/pull/43) ([aerokite](https://github.com/aerokite))
- Address status field changes [\#42](https://github.com/kubedb/postgres/pull/42) ([aerokite](https://github.com/aerokite))
- Use canary tag for k8sdb images [\#40](https://github.com/kubedb/postgres/pull/40) ([aerokite](https://github.com/aerokite))
- Install ca-certificates in operator docker image. [\#39](https://github.com/kubedb/postgres/pull/39) ([tamalsaha](https://github.com/tamalsaha))
- Add deployment.yaml [\#38](https://github.com/kubedb/postgres/pull/38) ([aerokite](https://github.com/aerokite))
- Rename "destroy" to "wipeOut" [\#36](https://github.com/kubedb/postgres/pull/36) ([tamalsaha](https://github.com/tamalsaha))
- Store Postgres Spec in DeletedDatabase [\#34](https://github.com/kubedb/postgres/pull/34) ([aerokite](https://github.com/aerokite))
- Update timing fields [\#33](https://github.com/kubedb/postgres/pull/33) ([tamalsaha](https://github.com/tamalsaha))
- Remove -v\* suffix from docker image [\#32](https://github.com/kubedb/postgres/pull/32) ([tamalsaha](https://github.com/tamalsaha))
- Use k8sdb docker hub account [\#31](https://github.com/kubedb/postgres/pull/31) ([tamalsaha](https://github.com/tamalsaha))
- Support initialization using DatabaseSnapshot [\#30](https://github.com/kubedb/postgres/pull/30) ([aerokite](https://github.com/aerokite))
- Use resource name constant from apimachinery [\#29](https://github.com/kubedb/postgres/pull/29) ([tamalsaha](https://github.com/tamalsaha))
- Use one controller struct [\#28](https://github.com/kubedb/postgres/pull/28) ([tamalsaha](https://github.com/tamalsaha))
- Implement updated interfaces. [\#27](https://github.com/kubedb/postgres/pull/27) ([tamalsaha](https://github.com/tamalsaha))
- Rename controller image to k8s-pg [\#26](https://github.com/kubedb/postgres/pull/26) ([tamalsaha](https://github.com/tamalsaha))
- Implement Snapshotter, Deleter with Controller [\#25](https://github.com/kubedb/postgres/pull/25) ([aerokite](https://github.com/aerokite))
- Implement recover operation [\#24](https://github.com/kubedb/postgres/pull/24) ([aerokite](https://github.com/aerokite))
- Implement k8sdb framework [\#23](https://github.com/kubedb/postgres/pull/23) ([aerokite](https://github.com/aerokite))
- Use osm to pull/push snapshots [\#22](https://github.com/kubedb/postgres/pull/22) ([aerokite](https://github.com/aerokite))
- Modify [\#19](https://github.com/kubedb/postgres/pull/19) ([aerokite](https://github.com/aerokite))
- Fix [\#18](https://github.com/kubedb/postgres/pull/18) ([aerokite](https://github.com/aerokite))
- Remove "volume.alpha.kubernetes.io/storage-class" annotation [\#14](https://github.com/kubedb/postgres/pull/14) ([aerokite](https://github.com/aerokite))
- add controller operation & docker files [\#2](https://github.com/kubedb/postgres/pull/2) ([aerokite](https://github.com/aerokite))
- Modify skeleton to postgres [\#1](https://github.com/kubedb/postgres/pull/1) ([aerokite](https://github.com/aerokite))



\* *This Change Log was automatically generated by [github_changelog_generator](https://github.com/skywinder/Github-Changelog-Generator)*