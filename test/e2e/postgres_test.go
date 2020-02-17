/*
Copyright The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"kubedb.dev/postgres/test/e2e/framework"
	"kubedb.dev/postgres/test/e2e/matcher"

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	core_util "kmodules.xyz/client-go/core/v1"
	store "kmodules.xyz/objectstore-api/api/v1"
	stashV1alpha1 "stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	stashV1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

const (
	S3_BUCKET_NAME          = "S3_BUCKET_NAME"
	GCS_BUCKET_NAME         = "GCS_BUCKET_NAME"
	AZURE_CONTAINER_NAME    = "AZURE_CONTAINER_NAME"
	SWIFT_CONTAINER_NAME    = "SWIFT_CONTAINER_NAME"
	POSTGRES_DB             = "POSTGRES_DB"
	POSTGRES_PASSWORD       = "POSTGRES_PASSWORD"
	PGDATA                  = "PGDATA"
	POSTGRES_USER           = "POSTGRES_USER"
	POSTGRES_INITDB_ARGS    = "POSTGRES_INITDB_ARGS"
	POSTGRES_INITDB_WALDIR  = "POSTGRES_INITDB_WALDIR"
	POSTGRES_INITDB_XLOGDIR = "POSTGRES_INITDB_XLOGDIR"
)

var _ = Describe("Postgres", func() {
	var (
		err                 error
		f                   *framework.Invocation
		postgres            *api.Postgres
		garbagePostgres     *api.PostgresList
		secret              *core.Secret
		skipMessage         string
		skipWalDataChecking bool
		skipMinioDeployment bool
		dbName              string
		dbUser              string
	)

	BeforeEach(func() {
		f = root.Invoke()
		postgres = f.Postgres()
		garbagePostgres = new(api.PostgresList)
		secret = nil
		skipMessage = ""
		skipWalDataChecking = true
		skipMinioDeployment = true
		dbName = "postgres"
		dbUser = "postgres"
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			f.PrintDebugHelpers()
		}
	})

	var createAndWaitForRunning = func() {
		By("Creating Postgres: " + postgres.Name)
		err = f.CreatePostgres(postgres)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running postgres")
		f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

		By("Wait for AppBinding to create")
		f.EventuallyAppBinding(postgres.ObjectMeta).Should(BeTrue())

		By("Check valid AppBinding Specs")
		err := f.CheckAppBindingSpec(postgres.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for database to be ready")
		f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())
	}

	var testGeneralBehaviour = func() {
		if skipMessage != "" {
			Skip(skipMessage)
		}
		// Create Postgres
		createAndWaitForRunning()

		By("Creating Schema")
		f.EventuallyCreateSchema(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

		By("Creating Table")
		f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

		By("Checking Table")
		f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

		By("Halt Postgres: Update postgres to set spec.halted = true")
		_, err := f.PatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
			in.Spec.Halted = true
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Wait for halted postgres")
		f.EventuallyPostgresPhase(postgres.ObjectMeta).Should(Equal(api.DatabasePhaseHalted))

		By("Resume Postgres: Update postgres to set spec.halted = false")
		_, err = f.PatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
			in.Spec.Halted = false
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running postgres")
		f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

		By("Checking Table")
		f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))
	}

	var shouldRunAndInsertData = func() {
		// Create and wait for running Postgres
		createAndWaitForRunning()

		By("Creating Schema")
		f.EventuallyCreateSchema(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

		By("Creating Table")
		f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

		By("Checking Table")
		f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))
	}

	var deleteTestResource = func() {
		if postgres == nil {
			Skip("Skipping")
		}

		By("Check if postgres " + postgres.Name + " exists.")
		mg, err := f.GetPostgres(postgres.ObjectMeta)
		if err != nil && kerr.IsNotFound(err) {
			// Postgres was not created. Hence, rest of cleanup is not necessary.
			return
		}
		Expect(err).NotTo(HaveOccurred())

		By("Update postgres to set spec.terminationPolicy = WipeOut")
		_, err = f.PatchPostgres(mg.ObjectMeta, func(in *api.Postgres) *api.Postgres {
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Delete postgres")
		err = f.DeletePostgres(postgres.ObjectMeta)
		if err != nil && kerr.IsNotFound(err) {
			// Postgres was not created. Hence, rest of cleanup is not necessary.
			return
		}
		Expect(err).NotTo(HaveOccurred())

		By("Wait for postgres to be deleted")
		f.EventuallyPostgres(postgres.ObjectMeta).Should(BeFalse())

		By("Wait for postgres resources to be wipedOut")
		f.EventuallyWipedOut(postgres.ObjectMeta).Should(Succeed())

		if postgres.Spec.Archiver != nil && !skipWalDataChecking {
			By("Checking wal data has been removed")
			f.EventuallyWalDataFound(postgres).Should(BeFalse())
		}
	}

	AfterEach(func() {
		// Delete test resource
		deleteTestResource()

		for i := len(garbagePostgres.Items) - 1; i >= 0; i-- {
			*postgres = garbagePostgres.Items[i]
			// Delete test resource
			deleteTestResource()
		}

		if secret != nil {
			err := f.DeleteSecret(secret.ObjectMeta)
			if !kerr.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}

		if !skipMinioDeployment {
			By("Deleting Minio Server")
			f.DeleteMinioServer()
		}
	})

	// if secret is empty (no .env file) then skip
	JustBeforeEach(func() {
		if secret != nil && len(secret.Data) == 0 &&
			(postgres.Spec.Archiver != nil && postgres.Spec.Archiver.Storage != nil && postgres.Spec.Archiver.Storage.Local == nil) {
			Skip("Missing repository credential")
		}
	})

	Describe("Test", func() {

		Context("General", func() {

			Context("With PVC", func() {

				It("should run successfully", testGeneralBehaviour)
			})

			Context("with custom SA Name", func() {
				BeforeEach(func() {
					customSecret := f.SecretForDatabaseAuthentication(postgres.ObjectMeta)
					postgres.Spec.DatabaseSecret = &core.SecretVolumeSource{
						SecretName: customSecret.Name,
					}
					err := f.CreateSecret(customSecret)
					Expect(err).NotTo(HaveOccurred())
					postgres.Spec.PodTemplate.Spec.ServiceAccountName = "my-custom-sa"
					postgres.Spec.TerminationPolicy = api.TerminationPolicyHalt
				})
				It("should start and resume successfully", func() {
					//shouldTakeSnapshot()
					createAndWaitForRunning()
					if postgres == nil {
						Skip("Skipping")
					}

					By("Check if Postgres " + postgres.Name + " exists.")
					_, err := f.GetPostgres(postgres.ObjectMeta)
					if err != nil {
						if kerr.IsNotFound(err) {
							// Postgres was not created. Hence, rest of cleanup is not necessary.
							return
						}
						Expect(err).NotTo(HaveOccurred())
					}

					By("Delete postgres: " + postgres.Name)
					err = f.DeletePostgres(postgres.ObjectMeta)
					if err != nil {
						if kerr.IsNotFound(err) {
							// Postgres was not created. Hence, rest of cleanup is not necessary.
							log.Infof("Skipping rest of cleanup. Reason: Postgres %s is not found.", postgres.Name)
							return
						}
						Expect(err).NotTo(HaveOccurred())
					}

					By("Wait for postgres to be deleted")
					f.EventuallyPostgres(postgres.ObjectMeta).Should(BeFalse())

					By("Resume PG")
					createAndWaitForRunning()
				})
			})

			Context("With Custom Resources", func() {

				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
				})
				Context("with custom SA", func() {
					var customSAForDB *core.ServiceAccount
					var customRoleForDB *rbac.Role
					var customRoleBindingForDB *rbac.RoleBinding
					var customSAForSnapshot *core.ServiceAccount
					var customRoleForSnapshot *rbac.Role
					var customRoleBindingForSnapshot *rbac.RoleBinding
					BeforeEach(func() {
						postgres.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

						customSAForDB = f.ServiceAccount()
						postgres.Spec.PodTemplate.Spec.ServiceAccountName = customSAForDB.Name
						customSAForSnapshot = f.ServiceAccount()

						customRoleForDB = f.RoleForPostgres(postgres.ObjectMeta)
						customRoleForSnapshot = f.RoleForSnapshot(postgres.ObjectMeta)

						customRoleBindingForDB = f.RoleBinding(customSAForDB.Name, customRoleForDB.Name)
						customRoleBindingForSnapshot = f.RoleBinding(customSAForSnapshot.Name, customRoleForSnapshot.Name)

						By("Create Database SA")
						err = f.CreateServiceAccount(customSAForDB)
						Expect(err).NotTo(HaveOccurred())
						By("Create Database Role")
						err = f.CreateRole(customRoleForDB)
						Expect(err).NotTo(HaveOccurred())
						By("Create Database RoleBinding")
						err = f.CreateRoleBinding(customRoleBindingForDB)
						Expect(err).NotTo(HaveOccurred())

						By("Create Snapshot SA")
						err = f.CreateServiceAccount(customSAForSnapshot)
						Expect(err).NotTo(HaveOccurred())
						By("Create Snapshot Role")
						err = f.CreateRole(customRoleForSnapshot)
						Expect(err).NotTo(HaveOccurred())
						By("Create Snapshot RoleBinding")
						err = f.CreateRoleBinding(customRoleBindingForSnapshot)
						Expect(err).NotTo(HaveOccurred())
					})
					It("should run successfully", func() {
						testGeneralBehaviour()
					})
				})

				Context("with custom Secret", func() {
					var customSecret *core.Secret
					BeforeEach(func() {

						customSecret = f.SecretForDatabaseAuthentication(postgres.ObjectMeta)
						postgres.Spec.DatabaseSecret = &core.SecretVolumeSource{
							SecretName: customSecret.Name,
						}

					})
					It("should preserve custom secret successfully", func() {
						By("Create Database Secret")
						err := f.CreateSecret(customSecret)
						Expect(err).NotTo(HaveOccurred())

						testGeneralBehaviour()
						deleteTestResource()
						By("Verifying Database Secret is present")
						err = f.CheckSecret(customSecret)
						Expect(err).NotTo(HaveOccurred())
					})
				})

			})

			Context("PDB", func() {

				It("should run evictions successfully", func() {
					// Create Postgres
					postgres.Spec.Replicas = types.Int32P(3)
					createAndWaitForRunning()
					//Evict a Postgres pod
					By("Try to evict Pods")
					err := f.EvictPodsFromStatefulSet(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("Initialize", func() {

			Context("With Script", func() {

				var configMap *core.ConfigMap

				BeforeEach(func() {
					configMap = f.ConfigMapForInitialization()
					err := f.CreateConfigMap(configMap)
					Expect(err).NotTo(HaveOccurred())

					postgres.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: configMap.Name,
									},
								},
							},
						},
					}
				})

				AfterEach(func() {
					err := f.DeleteConfigMap(configMap.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should run successfully", func() {
					// Create Postgres
					createAndWaitForRunning()

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(1))
				})

			})

			// To run this test,
			// 1st: Deploy stash latest operator
			// 2nd: create postgres related tasks and functions from
			// `kubedb.dev/postgres/hack/dev/examples/stash01_config.yaml`
			Context("With Stash/Restic", func() {
				var bc *stashV1beta1.BackupConfiguration
				var rs *stashV1beta1.RestoreSession
				var repo *stashV1alpha1.Repository

				BeforeEach(func() {
					if !f.FoundStashCRDs() {
						Skip("Skipping tests for stash integration. reason: stash operator is not running.")
					}
				})

				AfterEach(func() {
					By("Deleting RestoreSession")
					err = f.DeleteRestoreSession(rs.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Deleting Repository")
					err = f.DeleteRepository(repo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				var createAndWaitForInitializing = func() {
					By("Creating Postgres: " + postgres.Name)
					err = f.CreatePostgres(postgres)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Initializing postgres")
					f.EventuallyPostgresPhase(postgres.ObjectMeta).Should(Equal(api.DatabasePhaseInitializing))

					By("Wait for AppBinding to create")
					f.EventuallyAppBinding(postgres.ObjectMeta).Should(BeTrue())

					By("Check valid AppBinding Specs")
					err = f.CheckAppBindingSpec(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for database to be ready")
					f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())
				}

				var shouldInitializeFromStash = func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Creating Schema")
					f.EventuallyCreateSchema(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

					By("Creating Table")
					f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

					By("Create Secret")
					err = f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					By("Create Stash-Repositories")
					err = f.CreateRepository(repo)
					Expect(err).NotTo(HaveOccurred())

					By("Create Stash-BackupConfiguration")
					err = f.CreateBackupConfiguration(bc)
					Expect(err).NotTo(HaveOccurred())

					By("Check for snapshot count in stash-repository")
					f.EventuallySnapshotInRepository(repo.ObjectMeta).Should(matcher.MoreThan(2))

					By("Delete BackupConfiguration to stop backup scheduling")
					err = f.DeleteBackupConfiguration(bc.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

					By("Create postgres from stash")
					*postgres = *f.Postgres()
					rs = f.RestoreSession(postgres.ObjectMeta, repo)
					postgres.Spec.DatabaseSecret = oldPostgres.Spec.DatabaseSecret
					postgres.Spec.Init = &api.InitSpec{
						StashRestoreSession: &core.LocalObjectReference{
							Name: rs.Name,
						},
					}

					// Create and wait for running Postgres
					createAndWaitForInitializing()

					// wait few time before postgres is running after initial startup
					// TODO: fix this in operator. May be add sophisticated(!) readiness probe.
					time.Sleep(time.Minute * 2)
					By("Waiting for database to be ready")
					f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

					By("Create RestoreSession")
					err = f.CreateRestoreSession(rs)
					Expect(err).NotTo(HaveOccurred())

					// eventually backupsession succeeded
					By("Check for Succeeded restoreSession")
					f.EventuallyRestoreSessionPhase(rs.ObjectMeta).Should(Equal(stashV1beta1.RestoreSessionSucceeded))

					By("Wait for Running postgres")
					f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

					By("Waiting for database to be ready")
					f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))
				}

				Context("From GCS backend", func() {

					BeforeEach(func() {
						secret = f.SecretForGCSBackend()
						secret = f.PatchSecretForRestic(secret)
						repo = f.Repository(postgres.ObjectMeta)
						bc = f.BackupConfiguration(postgres.ObjectMeta, repo)

						repo.Spec.Backend = store.Backend{
							GCS: &store.GCSSpec{
								Bucket: os.Getenv("GCS_BUCKET_NAME"),
								Prefix: fmt.Sprintf("stash/%v/%v", postgres.Namespace, postgres.Name),
							},
							StorageSecretName: secret.Name,
						}
					})

					It("should run successfully", shouldInitializeFromStash)
				})
			})
		})

		Context("Resume", func() {
			var usedInitialized bool
			BeforeEach(func() {
				usedInitialized = false
			})

			var shouldResumeSuccessfully = func() {
				// Create and wait for running Postgres
				createAndWaitForRunning()

				By("Delete postgres")
				err := f.DeletePostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for postgres to be deleted")
				f.EventuallyPostgres(postgres.ObjectMeta).Should(BeFalse())

				// Create Postgres object again to resume it
				By("Create Postgres: " + postgres.Name)
				err = f.CreatePostgres(postgres)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for Running postgres")
				f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

				pg, err := f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				*postgres = *pg
				if usedInitialized {
					_, ok := postgres.Annotations[api.AnnotationInitialized]
					Expect(ok).Should(BeTrue())
				}
			}

			Context("-", func() {
				It("should resume DormantDatabase successfully", shouldResumeSuccessfully)
			})

			Context("With Init", func() {
				var configMap *core.ConfigMap

				BeforeEach(func() {
					configMap = f.ConfigMapForInitialization()
					err := f.CreateConfigMap(configMap)
					Expect(err).NotTo(HaveOccurred())

					postgres.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: configMap.Name,
									},
								},
							},
						},
					}
				})

				AfterEach(func() {
					err := f.DeleteConfigMap(configMap.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should resume DormantDatabase successfully", shouldResumeSuccessfully)
			})

			Context("Resume Multiple times - with init", func() {
				var configMap *core.ConfigMap

				BeforeEach(func() {
					configMap = f.ConfigMapForInitialization()
					err := f.CreateConfigMap(configMap)
					Expect(err).NotTo(HaveOccurred())

					usedInitialized = true
					postgres.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							ScriptPath: "postgres-init-scripts/run.sh",
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: configMap.Name,
									},
								},
							},
						},
					}
				})

				AfterEach(func() {
					err := f.DeleteConfigMap(configMap.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					for i := 0; i < 3; i++ {
						By(fmt.Sprintf("%v-th", i+1) + " time running.")
						By("Delete postgres")
						err := f.DeletePostgres(postgres.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for postgres to be halted")
						f.EventuallyPostgres(postgres.ObjectMeta).Should(BeFalse())

						// Create Postgres object again to resume it
						By("Create Postgres: " + postgres.Name)
						err = f.CreatePostgres(postgres)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for Running postgres")
						f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

						_, err = f.GetPostgres(postgres.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())
					}
				})
			})
		})

		Context("Archive with wal-g", func() {

			var postgres2nd, postgres3rd *api.Postgres

			BeforeEach(func() {
				secret = f.SecretForS3Backend()
				skipWalDataChecking = false
				postgres.Spec.Archiver = &api.PostgresArchiverSpec{
					Storage: &store.Backend{
						StorageSecretName: secret.Name,
						S3: &store.S3Spec{
							Bucket: os.Getenv(S3_BUCKET_NAME),
						},
					},
				}
			})

			archiveAndInitializeFromArchive := func() {
				// -- > 1st Postgres < --
				err := f.CreateSecret(secret)
				// Secret can be already created in Minio Tests
				if err != nil && !kerr.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}

				// Create Postgres
				createAndWaitForRunning()

				By("Creating Schema")
				f.EventuallyCreateSchema(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Creating Table")
				f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

				By("Checking Archive")
				f.EventuallyCountArchive(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Checking wal data in backend")
				f.EventuallyWalDataFound(postgres).Should(BeTrue())

				oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

				// -- > 1st Postgres end < --

				// -- > 2nd Postgres < --
				postgres2nd.Spec.DatabaseSecret = oldPostgres.Spec.DatabaseSecret
				*postgres = *postgres2nd

				// Create Postgres
				createAndWaitForRunning()

				By("Ping Database")
				f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Checking existing data in Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

				By("Creating Table")
				f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(6))

				By("Checking Archive")
				f.EventuallyCountArchive(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Checking wal data in backend")
				f.EventuallyWalDataFound(postgres).Should(BeTrue())

				oldPostgres, err = f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

				// -- > 2nd Postgres end < --

				// -- > 3rd Postgres < --
				postgres3rd.Spec.DatabaseSecret = oldPostgres.Spec.DatabaseSecret
				*postgres = *postgres3rd

				// Create Postgres
				createAndWaitForRunning()

				By("Ping Database")
				f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(6))
			}

			archiveAndInitializeFromLocalArchive := func() {
				// -- > 1st Postgres < --
				// Create Postgres
				createAndWaitForRunning()

				By("Creating Schema")
				f.EventuallyCreateSchema(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Creating Table")
				f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

				By("Checking Archive")
				f.EventuallyCountArchive(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

				// -- > 1st Postgres end < --

				// -- > 2nd Postgres < --
				postgres2nd.Spec.DatabaseSecret = oldPostgres.Spec.DatabaseSecret
				*postgres = *postgres2nd

				// Create Postgres
				createAndWaitForRunning()

				By("Ping Database")
				f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Creating Table")
				f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(6))

				By("Checking Archive")
				f.EventuallyCountArchive(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				oldPostgres, err = f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

				// -- > 2nd Postgres end < --

				// -- > 3rd Postgres < --
				postgres3rd.Spec.DatabaseSecret = oldPostgres.Spec.DatabaseSecret
				*postgres = *postgres3rd

				// Create Postgres
				createAndWaitForRunning()

				By("Ping Database")
				f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(6))

			}

			shouldWipeOutWalData := func() {

				err := f.CreateSecret(secret)
				Expect(err).NotTo(HaveOccurred())

				// Create Postgres
				createAndWaitForRunning()

				By("Creating Schema")
				f.EventuallyCreateSchema(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Creating Table")
				f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

				By("Checking Archive")
				f.EventuallyCountArchive(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Checking wal data in backend")
				f.EventuallyWalDataFound(postgres).Should(BeTrue())

				postgres, err = f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Deleting Postgres crd")
				err = f.DeletePostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("wait until postgres is deleted")
				f.EventuallyPostgres(postgres.ObjectMeta).Should(BeFalse())

				By("Checking PVCs has been deleted")
				f.EventuallyPVCCount(postgres.ObjectMeta).Should(Equal(0))

				By("Checking Secrets has been deleted")
				f.EventuallyDBSecretCount(postgres.ObjectMeta).Should(Equal(0))

				By("Checking Wal data removed from backend")
				f.EventuallyWalDataFound(postgres).Should(BeFalse())
			}

			// Archiving not working for local volume. xref: https://github.com/kubedb/project/issues/623
			// TODO: fix the issue and enable this test
			XContext("In Local", func() {
				BeforeEach(func() {
					skipWalDataChecking = true
				})

				Context("With PVC as Archive backend", func() {
					var firstPVC *core.PersistentVolumeClaim
					var secondPVC *core.PersistentVolumeClaim
					BeforeEach(func() {
						secret = f.SecretForLocalBackend()
						firstPVC = f.GetNamedPersistentVolumeClaim("first")
						secondPVC = f.GetNamedPersistentVolumeClaim("second")
						err = f.CreatePersistentVolumeClaim(firstPVC)
						Expect(err).NotTo(HaveOccurred())
						err = f.CreatePersistentVolumeClaim(secondPVC)
						Expect(err).NotTo(HaveOccurred())

						postgres.Spec.Archiver = &api.PostgresArchiverSpec{
							Storage: &store.Backend{
								Local: &store.LocalSpec{
									MountPath: "/walarchive",
									VolumeSource: core.VolumeSource{
										PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
											ClaimName: firstPVC.Name,
										},
									},
								},
							},
						}
						// 2nd Postgres
						postgres2nd = f.Postgres()
						postgres2nd.Spec.Archiver = &api.PostgresArchiverSpec{
							Storage: &store.Backend{
								Local: &store.LocalSpec{
									MountPath: "/walarchive",
									VolumeSource: core.VolumeSource{
										PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
											ClaimName: secondPVC.Name,
										},
									},
								},
							},
						}
						postgres2nd.Spec.Init = &api.InitSpec{
							PostgresWAL: &api.PostgresWALSourceSpec{
								Backend: store.Backend{
									Local: &store.LocalSpec{
										MountPath: "/walsource",
										SubPath:   fmt.Sprintf("%s-0", postgres.Name),
										VolumeSource: core.VolumeSource{
											PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
												ClaimName: firstPVC.Name,
											},
										},
									},
								},
							},
						}
						// -- > 3rd Postgres < --
						postgres3rd = f.Postgres()
						postgres3rd.Spec.Init = &api.InitSpec{
							PostgresWAL: &api.PostgresWALSourceSpec{
								Backend: store.Backend{
									Local: &store.LocalSpec{
										MountPath: "/final/source",
										SubPath:   fmt.Sprintf("%s-0", postgres2nd.Name),
										VolumeSource: core.VolumeSource{
											PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
												ClaimName: secondPVC.Name,
											},
										},
									},
								},
							},
						}

					})

					It("should archive and should resume from archive successfully", archiveAndInitializeFromLocalArchive)

				})
			})

			Context("Minio S3", func() {
				BeforeEach(func() {
					skipWalDataChecking = false
					skipMinioDeployment = false
				})

				Context("With ca-cert", func() {
					BeforeEach(func() {
						secret = f.SecretForMinioBackend()
						err := f.CreateSecret(secret)
						Expect(err).NotTo(HaveOccurred())

						By("Creating Minio server with cacert")
						addrs, err := f.CreateMinioServer(true, nil, secret)
						Expect(err).NotTo(HaveOccurred())

						postgres.Spec.Archiver = &api.PostgresArchiverSpec{
							Storage: &store.Backend{
								StorageSecretName: secret.Name,
								S3: &store.S3Spec{
									Bucket:   os.Getenv(S3_BUCKET_NAME),
									Endpoint: addrs,
								},
							},
						}

						// -- > 2nd Postgres < --
						postgres2nd = f.Postgres()
						postgres2nd.Spec.Archiver = &api.PostgresArchiverSpec{
							Storage: &store.Backend{
								StorageSecretName: secret.Name,
								S3: &store.S3Spec{
									Bucket:   os.Getenv(S3_BUCKET_NAME),
									Endpoint: addrs,
								},
							},
						}
						postgres2nd.Spec.Init = &api.InitSpec{
							PostgresWAL: &api.PostgresWALSourceSpec{
								Backend: store.Backend{
									StorageSecretName: secret.Name,
									S3: &store.S3Spec{
										Bucket:   os.Getenv(S3_BUCKET_NAME),
										Prefix:   fmt.Sprintf("kubedb/%s/%s/archive/", postgres.Namespace, postgres.Name),
										Endpoint: addrs,
									},
								},
							},
						}

						// -- > 3rd Postgres < --
						postgres3rd = f.Postgres()
						postgres3rd.Spec.Init = &api.InitSpec{
							PostgresWAL: &api.PostgresWALSourceSpec{
								Backend: store.Backend{
									StorageSecretName: secret.Name,
									S3: &store.S3Spec{
										Bucket:   os.Getenv(S3_BUCKET_NAME),
										Prefix:   fmt.Sprintf("kubedb/%s/%s/archive/", postgres2nd.Namespace, postgres2nd.Name),
										Endpoint: addrs,
									},
								},
							},
						}

					})

					It("should archive and should resume from archive successfully", archiveAndInitializeFromArchive)

				})

				Context("Without ca-cert", func() {
					BeforeEach(func() {
						secret = f.SecretForS3Backend()
						err := f.CreateSecret(secret)
						Expect(err).NotTo(HaveOccurred())

						By("Creating Minio server without cacert")
						addrs, err := f.CreateMinioServer(false, nil, secret)
						Expect(err).NotTo(HaveOccurred())

						postgres.Spec.Archiver = &api.PostgresArchiverSpec{
							Storage: &store.Backend{
								StorageSecretName: secret.Name,
								S3: &store.S3Spec{
									Bucket:   os.Getenv(S3_BUCKET_NAME),
									Endpoint: addrs,
								},
							},
						}

						// -- > 2nd Postgres < --
						postgres2nd = f.Postgres()
						postgres2nd.Spec.Archiver = &api.PostgresArchiverSpec{
							Storage: &store.Backend{
								StorageSecretName: secret.Name,
								S3: &store.S3Spec{
									Bucket:   os.Getenv(S3_BUCKET_NAME),
									Endpoint: addrs,
								},
							},
						}
						postgres2nd.Spec.Init = &api.InitSpec{
							PostgresWAL: &api.PostgresWALSourceSpec{
								Backend: store.Backend{
									StorageSecretName: secret.Name,
									S3: &store.S3Spec{
										Bucket:   os.Getenv(S3_BUCKET_NAME),
										Prefix:   fmt.Sprintf("kubedb/%s/%s/archive/", postgres.Namespace, postgres.Name),
										Endpoint: addrs,
									},
								},
							},
						}

						// -- > 3rd Postgres < --
						postgres3rd = f.Postgres()
						postgres3rd.Spec.Init = &api.InitSpec{
							PostgresWAL: &api.PostgresWALSourceSpec{
								Backend: store.Backend{
									StorageSecretName: secret.Name,
									S3: &store.S3Spec{
										Bucket:   os.Getenv(S3_BUCKET_NAME),
										Prefix:   fmt.Sprintf("kubedb/%s/%s/archive/", postgres2nd.Namespace, postgres2nd.Name),
										Endpoint: addrs,
									},
								},
							},
						}

					})

					It("should archive and should resume from archive successfully", archiveAndInitializeFromArchive)
				})
			})

			Context("In S3", func() {

				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					skipWalDataChecking = false
					postgres.Spec.Archiver = &api.PostgresArchiverSpec{
						Storage: &store.Backend{
							StorageSecretName: secret.Name,
							S3: &store.S3Spec{
								Bucket: os.Getenv(S3_BUCKET_NAME),
							},
						},
					}

					// -- > 2nd Postgres < --
					postgres2nd = f.Postgres()
					postgres2nd.Spec.Archiver = &api.PostgresArchiverSpec{
						Storage: &store.Backend{
							StorageSecretName: secret.Name,
							S3: &store.S3Spec{
								Bucket: os.Getenv(S3_BUCKET_NAME),
							},
						},
					}
					postgres2nd.Spec.Init = &api.InitSpec{
						PostgresWAL: &api.PostgresWALSourceSpec{
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								S3: &store.S3Spec{
									Bucket: os.Getenv(S3_BUCKET_NAME),
									Prefix: fmt.Sprintf("kubedb/%s/%s/archive/", postgres.Namespace, postgres.Name),
								},
							},
						},
					}

					// -- > 3rd Postgres < --
					postgres3rd = f.Postgres()
					postgres3rd.Spec.Init = &api.InitSpec{
						PostgresWAL: &api.PostgresWALSourceSpec{
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								S3: &store.S3Spec{
									Bucket: os.Getenv(S3_BUCKET_NAME),
									Prefix: fmt.Sprintf("kubedb/%s/%s/archive/", postgres2nd.Namespace, postgres2nd.Name),
								},
							},
						},
					}
				})

				Context("Archive and Initialize from wal archive", func() {

					It("should archive and should resume from archive successfully", archiveAndInitializeFromArchive)
				})

				Context("With dedicated Elasticsearch", func() {

					BeforeEach(func() {
						postgres.Spec.Replicas = types.Int32P(3)
						postgres2nd.Spec.Replicas = types.Int32P(3)
						postgres3rd.Spec.Replicas = types.Int32P(3)
					})

					It("should archive and should resume from archive successfully", archiveAndInitializeFromArchive)
				})

				Context("WipeOut wal data", func() {

					BeforeEach(func() {
						postgres.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})

					It("should remove wal data from backend", shouldWipeOutWalData)
				})
			})

			Context("In GCS", func() {

				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
					skipWalDataChecking = false
					postgres.Spec.Archiver = &api.PostgresArchiverSpec{
						Storage: &store.Backend{
							StorageSecretName: secret.Name,
							GCS: &store.GCSSpec{
								Bucket: os.Getenv(GCS_BUCKET_NAME),
							},
						},
					}

					// -- > 2nd Postgres < --
					postgres2nd = f.Postgres()
					postgres2nd.Spec.Archiver = &api.PostgresArchiverSpec{
						Storage: &store.Backend{
							StorageSecretName: secret.Name,
							GCS: &store.GCSSpec{
								Bucket: os.Getenv(GCS_BUCKET_NAME),
							},
						},
					}
					postgres2nd.Spec.Init = &api.InitSpec{
						PostgresWAL: &api.PostgresWALSourceSpec{
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								GCS: &store.GCSSpec{
									Bucket: os.Getenv(GCS_BUCKET_NAME),
									Prefix: fmt.Sprintf("kubedb/%s/%s/archive/", postgres.Namespace, postgres.Name),
								},
							},
						},
					}

					// -- > 3rd Postgres < --
					postgres3rd = f.Postgres()
					postgres3rd.Spec.Init = &api.InitSpec{
						PostgresWAL: &api.PostgresWALSourceSpec{
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								GCS: &store.GCSSpec{
									Bucket: os.Getenv(GCS_BUCKET_NAME),
									Prefix: fmt.Sprintf("kubedb/%s/%s/archive/", postgres2nd.Namespace, postgres2nd.Name),
								},
							},
						},
					}
				})

				Context("Archive and Initialize from wal archive", func() {

					It("should archive and should resume from archive successfully", archiveAndInitializeFromArchive)
				})

				Context("WipeOut wal data", func() {

					BeforeEach(func() {
						postgres.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})

					It("should remove wal data from backend", shouldWipeOutWalData)
				})
			})

			Context("In AZURE", func() {

				BeforeEach(func() {
					secret = f.SecretForAzureBackend()
					skipWalDataChecking = false
					postgres.Spec.Archiver = &api.PostgresArchiverSpec{
						Storage: &store.Backend{
							StorageSecretName: secret.Name,
							Azure: &store.AzureSpec{
								Container: os.Getenv(AZURE_CONTAINER_NAME),
							},
						},
					}

					// -- > 2nd Postgres < --
					postgres2nd = f.Postgres()
					postgres2nd.Spec.Archiver = &api.PostgresArchiverSpec{
						Storage: &store.Backend{
							StorageSecretName: secret.Name,
							Azure: &store.AzureSpec{
								Container: os.Getenv(AZURE_CONTAINER_NAME),
							},
						},
					}
					postgres2nd.Spec.Init = &api.InitSpec{
						PostgresWAL: &api.PostgresWALSourceSpec{
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								Azure: &store.AzureSpec{
									Container: os.Getenv(AZURE_CONTAINER_NAME),
									Prefix:    fmt.Sprintf("kubedb/%s/%s/archive/", postgres.Namespace, postgres.Name),
								},
							},
						},
					}

					// -- > 3rd Postgres < --
					postgres3rd = f.Postgres()
					postgres3rd.Spec.Init = &api.InitSpec{
						PostgresWAL: &api.PostgresWALSourceSpec{
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								Azure: &store.AzureSpec{
									Container: os.Getenv(AZURE_CONTAINER_NAME),
									Prefix:    fmt.Sprintf("kubedb/%s/%s/archive/", postgres2nd.Namespace, postgres2nd.Name),
								},
							},
						},
					}
				})

				Context("Archive and Initialize from wal archive", func() {

					It("should archive and should resume from archive successfully", archiveAndInitializeFromArchive)
				})

				Context("WipeOut wal data", func() {

					BeforeEach(func() {
						postgres.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})

					It("should remove wal data from backend", shouldWipeOutWalData)
				})
			})

			Context("In SWIFT", func() {

				BeforeEach(func() {
					secret = f.SecretForSwiftBackend()
					skipWalDataChecking = false
					postgres.Spec.Archiver = &api.PostgresArchiverSpec{
						Storage: &store.Backend{
							StorageSecretName: secret.Name,
							Swift: &store.SwiftSpec{
								Container: os.Getenv(SWIFT_CONTAINER_NAME),
							},
						},
					}

					// -- > 2nd Postgres < --
					postgres2nd = f.Postgres()
					postgres2nd.Spec.Archiver = &api.PostgresArchiverSpec{
						Storage: &store.Backend{
							StorageSecretName: secret.Name,
							Swift: &store.SwiftSpec{
								Container: os.Getenv(SWIFT_CONTAINER_NAME),
							},
						},
					}
					postgres2nd.Spec.Init = &api.InitSpec{
						PostgresWAL: &api.PostgresWALSourceSpec{
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								Swift: &store.SwiftSpec{
									Container: os.Getenv(SWIFT_CONTAINER_NAME),
									Prefix:    fmt.Sprintf("kubedb/%s/%s/archive/", postgres.Namespace, postgres.Name),
								},
							},
						},
					}

					// -- > 3rd Postgres < --
					postgres3rd = f.Postgres()
					postgres3rd.Spec.Init = &api.InitSpec{
						PostgresWAL: &api.PostgresWALSourceSpec{
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								Swift: &store.SwiftSpec{
									Container: os.Getenv(SWIFT_CONTAINER_NAME),
									Prefix:    fmt.Sprintf("kubedb/%s/%s/archive/", postgres2nd.Namespace, postgres2nd.Name),
								},
							},
						},
					}
				})

				Context("Archive and Initialize from wal archive", func() {

					It("should archive and should resume from archive successfully", archiveAndInitializeFromArchive)
				})

				Context("WipeOut wal data", func() {

					BeforeEach(func() {
						postgres.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})

					It("should remove wal data from backend", shouldWipeOutWalData)
				})
			})
		})

		Context("Termination Policy", func() {

			BeforeEach(func() {
				secret = f.SecretForGCSBackend()
			})

			Context("with TerminationPolicyDoNotTerminate", func() {

				BeforeEach(func() {
					postgres.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
				})

				It("should work successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Delete postgres")
					err = f.DeletePostgres(postgres.ObjectMeta)
					Expect(err).Should(HaveOccurred())

					By("Postgres is not halted. Check for postgres")
					f.EventuallyPostgres(postgres.ObjectMeta).Should(BeTrue())

					By("Check for Running postgres")
					f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

					By("Update postgres to set spec.terminationPolicy = Halt")
					_, err := f.PatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
						return in
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("with TerminationPolicyHalt", func() {

				It("should create DormantDatabase and resume from it", func() {
					shouldRunAndInsertData()
					By("Halt Postgres: Update postgres to set spec.halted = true")
					_, err := f.PatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
						in.Spec.Halted = true
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for halted postgres")
					f.EventuallyPostgresPhase(postgres.ObjectMeta).Should(Equal(api.DatabasePhaseHalted))

					By("Resume Postgres: Update postgres to set spec.halted = false")
					_, err = f.PatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
						in.Spec.Halted = false
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Running postgres")
					f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))
				})
			})

			Context("with TerminationPolicyDelete", func() {

				BeforeEach(func() {
					postgres.Spec.TerminationPolicy = api.TerminationPolicyDelete
				})

				It("should not create DormantDatabase and should not delete secret and snapshot", func() {
					createAndWaitForRunning()

					By("Delete postgres")
					err = f.DeletePostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("wait until postgres is deleted")
					f.EventuallyPostgres(postgres.ObjectMeta).Should(BeFalse())

					By("Checking PVC has been deleted")
					f.EventuallyPVCCount(postgres.ObjectMeta).Should(Equal(0))

					By("Checking Secret hasn't been deleted")
					f.EventuallyDBSecretCount(postgres.ObjectMeta).Should(Equal(1))
				})
			})

			Context("with TerminationPolicyWipeOut", func() {

				BeforeEach(func() {
					postgres.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})

				It("should not create DormantDatabase and should wipeOut all", func() {
					// Run Postgres and take snapshot
					createAndWaitForRunning()

					By("Delete postgres")
					err = f.DeletePostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("wait until postgres is deleted")
					f.EventuallyPostgres(postgres.ObjectMeta).Should(BeFalse())

					By("Checking PVCs has been deleted")
					f.EventuallyPVCCount(postgres.ObjectMeta).Should(Equal(0))

					By("Checking Secrets has been deleted")
					f.EventuallyDBSecretCount(postgres.ObjectMeta).Should(Equal(0))
				})
			})
		})

		Context("EnvVars", func() {

			Context("With all supported EnvVars", func() {

				It("should create DB with provided EnvVars", func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}

					const (
						dataDir = "/var/pv/pgdata"
						walDir  = "/var/pv/wal"
					)
					dbName = f.App()
					postgres.Spec.PodTemplate.Spec.Env = []core.EnvVar{
						{
							Name:  PGDATA,
							Value: dataDir,
						},
						{
							Name:  POSTGRES_DB,
							Value: dbName,
						},
						{
							Name:  POSTGRES_INITDB_ARGS,
							Value: "--data-checksums",
						},
					}

					walEnv := []core.EnvVar{
						{
							Name:  POSTGRES_INITDB_WALDIR,
							Value: walDir,
						},
					}

					if strings.HasPrefix(framework.DBCatalogName, "9") {
						walEnv = []core.EnvVar{
							{
								Name:  POSTGRES_INITDB_XLOGDIR,
								Value: walDir,
							},
						}
					}
					postgres.Spec.PodTemplate.Spec.Env = core_util.UpsertEnvVars(postgres.Spec.PodTemplate.Spec.Env, walEnv...)

					// Run Postgres with provided Environment Variables
					testGeneralBehaviour()
				})
			})

			Context("Root Password as EnvVar", func() {

				It("should reject to create Postgres CRD", func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}

					dbName = f.App()
					postgres.Spec.PodTemplate.Spec.Env = []core.EnvVar{
						{
							Name:  POSTGRES_PASSWORD,
							Value: "not@secret",
						},
					}

					By("Creating Posgres: " + postgres.Name)
					err = f.CreatePostgres(postgres)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("Update EnvVar", func() {

				It("should not reject to update EnvVar", func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}

					dbName = f.App()
					postgres.Spec.PodTemplate.Spec.Env = []core.EnvVar{
						{
							Name:  POSTGRES_DB,
							Value: dbName,
						},
					}

					// Run Postgres with provided Environment Variables
					testGeneralBehaviour()

					By("Patching EnvVar")
					_, _, err = util.PatchPostgres(f.ExtClient().KubedbV1alpha1(), postgres, func(in *api.Postgres) *api.Postgres {
						in.Spec.PodTemplate.Spec.Env = []core.EnvVar{
							{
								Name:  POSTGRES_DB,
								Value: "patched-db",
							},
						}
						return in
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("Custom config", func() {

			customConfigs := []string{
				"shared_buffers=256MB",
				"max_connections=300",
			}

			Context("from configMap", func() {
				var userConfig *core.ConfigMap

				BeforeEach(func() {
					userConfig = f.GetCustomConfig(customConfigs)
				})

				AfterEach(func() {
					By("Deleting configMap: " + userConfig.Name)
					err := f.DeleteConfigMap(userConfig.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should set configuration provided in configMap", func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}

					By("Creating configMap: " + userConfig.Name)
					err := f.CreateConfigMap(userConfig)
					Expect(err).NotTo(HaveOccurred())

					postgres.Spec.ConfigSource = &core.VolumeSource{
						ConfigMap: &core.ConfigMapVolumeSource{
							LocalObjectReference: core.LocalObjectReference{
								Name: userConfig.Name,
							},
						},
					}

					// Create Postgres
					createAndWaitForRunning()

					By("Checking postgres configured from provided custom configuration")
					for _, cfg := range customConfigs {
						f.EventuallyPGSettings(postgres.ObjectMeta, dbName, dbUser, cfg).Should(matcher.Use(cfg))
					}
				})
			})
		})

		Context("StorageType ", func() {

			var shouldRunSuccessfully = func() {
				if skipMessage != "" {
					Skip(skipMessage)
				}
				// Create Postgres
				createAndWaitForRunning()

				By("Creating Schema")
				f.EventuallyCreateSchema(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Creating Table")
				f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))
			}

			Context("Ephemeral", func() {

				BeforeEach(func() {
					postgres.Spec.StorageType = api.StorageTypeEphemeral
					postgres.Spec.Storage = nil
				})

				Context("General Behaviour", func() {

					BeforeEach(func() {
						postgres.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})

					It("should run successfully", shouldRunSuccessfully)
				})

				Context("With TerminationPolicyHalt", func() {

					BeforeEach(func() {
						postgres.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})

					It("should reject to create Postgres object", func() {
						By("Creating Postgres: " + postgres.Name)
						err := f.CreatePostgres(postgres)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})
	})
})
