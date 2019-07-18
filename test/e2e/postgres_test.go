package e2e_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	core_util "kmodules.xyz/client-go/core/v1"
	meta_util "kmodules.xyz/client-go/meta"
	store "kmodules.xyz/objectstore-api/api/v1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"kubedb.dev/postgres/test/e2e/framework"
	"kubedb.dev/postgres/test/e2e/matcher"
	stashV1alpha1 "stash.appscode.dev/stash/apis/stash/v1alpha1"
	stashV1beta1 "stash.appscode.dev/stash/apis/stash/v1beta1"
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
		err                      error
		f                        *framework.Invocation
		postgres                 *api.Postgres
		garbagePostgres          *api.PostgresList
		snapshot                 *api.Snapshot
		secret                   *core.Secret
		skipMessage              string
		skipSnapshotDataChecking bool
		skipWalDataChecking      bool
		skipMinioDeployment      bool
		dbName                   string
		dbUser                   string
	)

	BeforeEach(func() {
		f = root.Invoke()
		postgres = f.Postgres()
		garbagePostgres = new(api.PostgresList)
		snapshot = f.Snapshot()
		secret = nil
		skipMessage = ""
		skipSnapshotDataChecking = true
		skipWalDataChecking = true
		skipMinioDeployment = true
		dbName = "postgres"
		dbUser = "postgres"
	})

	var createAndWaitForRunning = func() {
		By("Creating Postgres: " + postgres.Name)
		err = f.CreatePostgres(postgres)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running postgres")
		f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

		By("Waiting for database to be ready")
		f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

		By("Wait for AppBinding to create")
		f.EventuallyAppBinding(postgres.ObjectMeta).Should(BeTrue())

		By("Check valid AppBinding Specs")
		err := f.CheckAppBindingSpec(postgres.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())
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

		By("Delete postgres")
		err = f.DeletePostgres(postgres.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for postgres to be paused")
		f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

		// Create Postgres object again to resume it
		By("Create Postgres: " + postgres.Name)
		err = f.CreatePostgres(postgres)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for DormantDatabase to be deleted")
		f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

		By("Wait for Running postgres")
		f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

		By("Checking Table")
		f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))
	}

	var shouldTakeSnapshot = func() {
		// Create and wait for running Postgres
		createAndWaitForRunning()

		By("Create Secret")
		err := f.CreateSecret(secret)
		Expect(err).NotTo(HaveOccurred())

		By("Create Snapshot")
		err = f.CreateSnapshot(snapshot)
		Expect(err).NotTo(HaveOccurred())

		By("Check for Succeeded snapshot")
		f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

		if !skipSnapshotDataChecking {
			By("Check for snapshot data")
			f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
		}
	}

	var shouldInsertDataAndTakeSnapshot = func() {
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

		By("Create Snapshot")
		err = f.CreateSnapshot(snapshot)
		Expect(err).NotTo(HaveOccurred())

		By("Check for Succeeded snapshot")
		f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

		if !skipSnapshotDataChecking {
			By("Check for snapshot data")
			f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
		}
	}

	var deleteTestResource = func() {
		if postgres == nil {
			Skip("Skipping")
		}

		By("Check if Postgres " + postgres.Name + " exists.")
		pg, err := f.GetPostgres(postgres.ObjectMeta)
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

		if pg.Spec.TerminationPolicy == api.TerminationPolicyPause {

			By("Wait for postgres to be paused")
			f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

			By("Set DormantDatabase Spec.WipeOut to true")
			_, err := f.PatchDormantDatabase(postgres.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
				in.Spec.WipeOut = true
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Delete Dormant Database")
			err = f.DeleteDormantDatabase(postgres.ObjectMeta)
			if !kerr.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}

		}

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

		for _, pg := range garbagePostgres.Items {
			*postgres = pg
			// Delete test resource
			deleteTestResource()
		}

		if !skipSnapshotDataChecking {
			By("Check for snapshot data")
			f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
		}

		if secret != nil {
			err := f.DeleteSecret(secret.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		}

		if !skipMinioDeployment {
			By("Deleting Minio Server")
			err = f.DeleteMinioServer()
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("Test", func() {

		Context("General", func() {

			Context("With PVC", func() {

				It("should run successfully", testGeneralBehaviour)
			})

			Context("with custom SA Name", func() {
				BeforeEach(func() {
					var customSecret *core.Secret
					customSecret = f.SecretForDatabaseAuthentication(postgres.ObjectMeta)
					postgres.Spec.DatabaseSecret = &core.SecretVolumeSource{
						SecretName: customSecret.Name,
					}
					err := f.CreateSecret(customSecret)
					Expect(err).NotTo(HaveOccurred())
					postgres.Spec.PodTemplate.Spec.ServiceAccountName = "my-custom-sa"
					postgres.Spec.TerminationPolicy = api.TerminationPolicyPause
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

					By("Wait for postgres to be paused")
					f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

					By("Resume PG")
					createAndWaitForRunning()
				})
			})

			Context("With Custom Resources", func() {

				BeforeEach(func() {
					skipSnapshotDataChecking = false
					snapshot.Spec.DatabaseName = postgres.Name
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &store.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
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
						snapshot.Spec.PodTemplate.Spec.ServiceAccountName = customSAForSnapshot.Name

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
					It("should take snapshot successfully", func() {
						shouldTakeSnapshot()
					})
					It("should init from snapshot successfully", func() {
						By("Start initializing From Snapshot")
						// create postgres and take snapshot
						shouldInsertDataAndTakeSnapshot()

						oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

						By("Create postgres from snapshot")
						*postgres = *f.Postgres()
						postgres.Spec.DatabaseSecret = oldPostgres.Spec.DatabaseSecret
						postgres.Spec.Init = &api.InitSpec{
							SnapshotSource: &api.SnapshotSourceSpec{
								Namespace: snapshot.Namespace,
								Name:      snapshot.Name,
							},
						}

						//Create New role and bind it to existing SA
						By("Get new Role and RB")
						customRoleForReplayDB := f.RoleForPostgres(postgres.ObjectMeta)
						customRoleBindingForReplayDB := f.RoleBinding(customSAForDB.Name, customRoleForReplayDB.Name)

						By("Create Database Role")
						err = f.CreateRole(customRoleForReplayDB)
						Expect(err).NotTo(HaveOccurred())
						By("Create Database RoleBinding")
						err = f.CreateRoleBinding(customRoleBindingForReplayDB)
						Expect(err).NotTo(HaveOccurred())

						postgres.Spec.PodTemplate.Spec.ServiceAccountName = customSAForDB.Name

						// Create and wait for running Postgres
						createAndWaitForRunning()

						By("Check Table")
						f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))
					})

				})

				Context("with custom Secret", func() {
					var kubedbSecret *core.Secret
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

						shouldTakeSnapshot()
						deleteTestResource()
						By("Verifying Database Secret is present")
						err = f.CheckSecret(customSecret)
						Expect(err).NotTo(HaveOccurred())
					})
					It("should delete labelled secret successfully", func() {
						kubedbSecret = f.SecretForDatabaseAuthenticationWithLabel(postgres.ObjectMeta)
						postgres.Spec.DatabaseSecret = &core.SecretVolumeSource{
							SecretName: kubedbSecret.Name,
						}
						By("Create Database Secret")
						err = f.CreateSecret(kubedbSecret)
						Expect(err).NotTo(HaveOccurred())

						shouldTakeSnapshot()
						deleteTestResource()
						By("Verify Database Secret is not present")
						err = f.CheckSecret(kubedbSecret)
						Expect(err).To(HaveOccurred())
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

		Context("Snapshot", func() {

			BeforeEach(func() {
				skipSnapshotDataChecking = false
				snapshot.Spec.DatabaseName = postgres.Name
			})

			Context("In Local", func() {

				BeforeEach(func() {
					skipSnapshotDataChecking = true
					secret = f.SecretForLocalBackend()
					snapshot.Spec.StorageSecretName = secret.Name
				})

				Context("With EmptyDir as Snapshot's backend", func() {
					BeforeEach(func() {
						snapshot.Spec.Local = &store.LocalSpec{
							MountPath: "/repo",
							VolumeSource: core.VolumeSource{
								EmptyDir: &core.EmptyDirVolumeSource{},
							},
						}
					})

					It("should take Snapshot successfully", shouldTakeSnapshot)
				})

				Context("With PVC as Snapshot's backend", func() {
					var snapPVC *core.PersistentVolumeClaim

					BeforeEach(func() {
						snapPVC = f.GetPersistentVolumeClaim()
						err := f.CreatePersistentVolumeClaim(snapPVC)
						Expect(err).NotTo(HaveOccurred())

						snapshot.Spec.Local = &store.LocalSpec{
							MountPath: "/repo",
							VolumeSource: core.VolumeSource{
								PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
									ClaimName: snapPVC.Name,
								},
							},
						}
					})

					AfterEach(func() {
						err := f.DeletePersistentVolumeClaim(snapPVC.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should delete Snapshot successfully", func() {
						shouldTakeSnapshot()

						By("Deleting Snapshot")
						err := f.DeleteSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting Snapshot to be deleted")
						f.EventuallySnapshot(snapshot.ObjectMeta).Should(BeFalse())
					})
				})
			})

			Context("In S3", func() {

				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &store.S3Spec{
						Bucket: os.Getenv(S3_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)

				Context("Delete One Snapshot keeping others", func() {

					BeforeEach(func() {
						postgres.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/postgres-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})

					It("Delete One Snapshot keeping others", func() {
						// Create Postgres and take Snapshot
						shouldTakeSnapshot()

						oldSnapshot := snapshot.DeepCopy()

						// New snapshot that has old snapshot's name in prefix
						snapshot.Name += "-2"

						By(fmt.Sprintf("Create Snapshot %v", snapshot.Name))
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for Succeeded snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

						if !skipSnapshotDataChecking {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}

						// delete old snapshot
						By(fmt.Sprintf("Delete old Snapshot %v", snapshot.Name))
						err = f.DeleteSnapshot(oldSnapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for old Snapshot to be deleted")
						f.EventuallySnapshot(oldSnapshot.ObjectMeta).Should(BeFalse())
						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for old snapshot %v", oldSnapshot.Name))
							f.EventuallySnapshotDataFound(oldSnapshot).Should(BeFalse())
						}

						// check remaining snapshot
						By(fmt.Sprintf("Checking another Snapshot %v still exists", snapshot.Name))
						_, err = f.GetSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for remaining snapshot %v", snapshot.Name))
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}
					})
				})
			})

			Context("In GCS", func() {

				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &store.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)

				Context("faulty snapshot", func() {
					BeforeEach(func() {
						skipSnapshotDataChecking = true
						snapshot.Spec.StorageSecretName = secret.Name
						snapshot.Spec.GCS = &store.GCSSpec{
							Bucket: "nonexisting",
						}
					})
					It("snapshot should fail", func() {
						// Create and wait for running db
						createAndWaitForRunning()

						By("Create Secret")
						err := f.CreateSecret(secret)
						Expect(err).NotTo(HaveOccurred())

						By("Create Snapshot")
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for failed snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseFailed))
					})
				})

				Context("with custom username and password", func() {
					var customSecret *core.Secret
					BeforeEach(func() {
						customSecret = f.SecretForDatabaseAuthentication(postgres.ObjectMeta)
						postgres.Spec.DatabaseSecret = &core.SecretVolumeSource{
							SecretName: customSecret.Name,
						}
					})
					It("should take snapshot successfully", func() {
						By("Create Database Secret")
						err := f.CreateSecret(customSecret)
						Expect(err).NotTo(HaveOccurred())

						shouldTakeSnapshot()
					})
				})

				Context("Delete One Snapshot keeping others", func() {

					BeforeEach(func() {
						postgres.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/postgres-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})

					It("Delete One Snapshot keeping others", func() {
						// Create Postgres and take Snapshot
						shouldTakeSnapshot()

						oldSnapshot := snapshot.DeepCopy()

						// New snapshot that has old snapshot's name in prefix
						snapshot.Name += "-2"

						By(fmt.Sprintf("Create Snapshot %v", snapshot.Name))
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for Succeeded snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

						if !skipSnapshotDataChecking {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}

						// delete old snapshot
						By(fmt.Sprintf("Delete old Snapshot %v", snapshot.Name))
						err = f.DeleteSnapshot(oldSnapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for old Snapshot to be deleted")
						f.EventuallySnapshot(oldSnapshot.ObjectMeta).Should(BeFalse())
						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for old snapshot %v", oldSnapshot.Name))
							f.EventuallySnapshotDataFound(oldSnapshot).Should(BeFalse())
						}

						// check remaining snapshot
						By(fmt.Sprintf("Checking another Snapshot %v still exists", snapshot.Name))
						_, err = f.GetSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for remaining snapshot %v", snapshot.Name))
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}
					})
				})
			})

			Context("In Azure", func() {

				BeforeEach(func() {
					secret = f.SecretForAzureBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Azure = &store.AzureSpec{
						Container: os.Getenv(AZURE_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)

				Context("Delete One Snapshot keeping others", func() {

					BeforeEach(func() {
						postgres.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/postgres-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})

					It("Delete One Snapshot keeping others", func() {
						// Create Postgres and take Snapshot
						shouldTakeSnapshot()

						oldSnapshot := snapshot.DeepCopy()

						// New snapshot that has old snapshot's name in prefix
						snapshot.Name += "-2"

						By(fmt.Sprintf("Create Snapshot %v", snapshot.Name))
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for Succeeded snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

						if !skipSnapshotDataChecking {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}

						// delete old snapshot
						By(fmt.Sprintf("Delete old Snapshot %v", snapshot.Name))
						err = f.DeleteSnapshot(oldSnapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for old Snapshot to be deleted")
						f.EventuallySnapshot(oldSnapshot.ObjectMeta).Should(BeFalse())
						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for old snapshot %v", oldSnapshot.Name))
							f.EventuallySnapshotDataFound(oldSnapshot).Should(BeFalse())
						}

						// check remaining snapshot
						By(fmt.Sprintf("Checking another Snapshot %v still exists", snapshot.Name))
						_, err = f.GetSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for remaining snapshot %v", snapshot.Name))
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}
					})
				})
			})

			Context("In Swift", func() {

				BeforeEach(func() {
					secret = f.SecretForSwiftBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Swift = &store.SwiftSpec{
						Container: os.Getenv(SWIFT_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("Snapshot PodVolume Template - In S3", func() {

				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &store.S3Spec{
						Bucket: os.Getenv(S3_BUCKET_NAME),
					}
				})

				var shouldHandleJobVolumeSuccessfully = func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Get Postgres")
					es, err := f.GetPostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
					postgres.Spec = es.Spec

					By("Create Secret")
					err = f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					// determine pvcSpec and storageType for job
					// start
					pvcSpec := snapshot.Spec.PodVolumeClaimSpec
					if pvcSpec == nil {
						pvcSpec = postgres.Spec.Storage
					}
					st := snapshot.Spec.StorageType
					if st == nil {
						st = &postgres.Spec.StorageType
					}
					Expect(st).NotTo(BeNil())
					// end

					By("Create Snapshot")
					err = f.CreateSnapshot(snapshot)
					if *st == api.StorageTypeDurable && pvcSpec == nil {
						By("Create Snapshot should have failed")
						Expect(err).Should(HaveOccurred())
						return
					} else {
						Expect(err).NotTo(HaveOccurred())
					}

					By("Get Snapshot")
					snap, err := f.GetSnapshot(snapshot.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
					snapshot.Spec = snap.Spec

					if *st == api.StorageTypeEphemeral {
						storageSize := "0"
						if pvcSpec != nil {
							if sz, found := pvcSpec.Resources.Requests[core.ResourceStorage]; found {
								storageSize = sz.String()
							}
						}
						By(fmt.Sprintf("Check for Job Empty volume size: %v", storageSize))
						f.EventuallyJobVolumeEmptyDirSize(snapshot.ObjectMeta).Should(Equal(storageSize))
					} else if *st == api.StorageTypeDurable {
						sz, found := pvcSpec.Resources.Requests[core.ResourceStorage]
						Expect(found).NotTo(BeFalse())

						By("Check for Job PVC Volume size from snapshot")
						f.EventuallyJobPVCSize(snapshot.ObjectMeta).Should(Equal(sz.String()))
					}

					By("Check for succeeded snapshot")
					f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

					if !skipSnapshotDataChecking {
						By("Check for snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
					}
				}

				// db StorageType Scenarios
				// ==============> Start
				var dbStorageTypeScenarios = func() {
					Context("DBStorageType - Durable", func() {
						BeforeEach(func() {
							postgres.Spec.StorageType = api.StorageTypeDurable
							postgres.Spec.Storage = &core.PersistentVolumeClaimSpec{
								Resources: core.ResourceRequirements{
									Requests: core.ResourceList{
										core.ResourceStorage: resource.MustParse(framework.DBPvcStorageSize),
									},
								},
								StorageClassName: types.StringP(root.StorageClass),
							}

						})

						It("should Handle Job Volume Successfully", shouldHandleJobVolumeSuccessfully)
					})

					Context("DBStorageType - Ephemeral", func() {
						BeforeEach(func() {
							postgres.Spec.StorageType = api.StorageTypeEphemeral
							postgres.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						})

						Context("DBPvcSpec is nil", func() {
							BeforeEach(func() {
								postgres.Spec.Storage = nil
							})

							It("should Handle Job Volume Successfully", shouldHandleJobVolumeSuccessfully)
						})

						Context("DBPvcSpec is given [not nil]", func() {
							BeforeEach(func() {
								postgres.Spec.Storage = &core.PersistentVolumeClaimSpec{
									Resources: core.ResourceRequirements{
										Requests: core.ResourceList{
											core.ResourceStorage: resource.MustParse(framework.DBPvcStorageSize),
										},
									},
									StorageClassName: types.StringP(root.StorageClass),
								}
							})

							It("should Handle Job Volume Successfully", shouldHandleJobVolumeSuccessfully)
						})
					})
				}
				// End <==============

				// Snapshot PVC Scenarios
				// ==============> Start
				var snapshotPvcScenarios = func() {
					Context("Snapshot PVC is given [not nil]", func() {
						BeforeEach(func() {
							snapshot.Spec.PodVolumeClaimSpec = &core.PersistentVolumeClaimSpec{
								Resources: core.ResourceRequirements{
									Requests: core.ResourceList{
										core.ResourceStorage: resource.MustParse(framework.JobPvcStorageSize),
									},
								},
								StorageClassName: types.StringP(root.StorageClass),
							}
						})

						dbStorageTypeScenarios()
					})

					Context("Snapshot PVC is nil", func() {
						BeforeEach(func() {
							snapshot.Spec.PodVolumeClaimSpec = nil
						})

						dbStorageTypeScenarios()
					})
				}
				// End <==============

				Context("Snapshot StorageType is nil", func() {
					BeforeEach(func() {
						snapshot.Spec.StorageType = nil
					})

					snapshotPvcScenarios()
				})

				Context("Snapshot StorageType is Ephemeral", func() {
					BeforeEach(func() {
						ephemeral := api.StorageTypeEphemeral
						snapshot.Spec.StorageType = &ephemeral
					})

					snapshotPvcScenarios()
				})

				Context("Snapshot StorageType is Durable", func() {
					BeforeEach(func() {
						durable := api.StorageTypeDurable
						snapshot.Spec.StorageType = &durable
					})

					snapshotPvcScenarios()
				})
			})
		})

		Context("Initialize", func() {

			Context("With Script", func() {

				BeforeEach(func() {
					postgres.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/postgres-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
				})

				It("should run successfully", func() {
					// Create Postgres
					createAndWaitForRunning()

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(1))
				})

			})

			Context("With Snapshot", func() {

				var shouldInitializeFromSnapshot = func() {
					// create postgres and take snapshot
					shouldInsertDataAndTakeSnapshot()

					oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

					By("Create postgres from snapshot")
					*postgres = *f.Postgres()
					postgres.Spec.DatabaseSecret = oldPostgres.Spec.DatabaseSecret
					postgres.Spec.Init = &api.InitSpec{
						SnapshotSource: &api.SnapshotSourceSpec{
							Namespace: snapshot.Namespace,
							Name:      snapshot.Name,
						},
					}

					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))
				}

				Context("From Local backend", func() {
					var snapPVC *core.PersistentVolumeClaim

					BeforeEach(func() {

						skipSnapshotDataChecking = true
						snapPVC = f.GetPersistentVolumeClaim()
						err := f.CreatePersistentVolumeClaim(snapPVC)
						Expect(err).NotTo(HaveOccurred())

						secret = f.SecretForLocalBackend()
						snapshot.Spec.DatabaseName = postgres.Name
						snapshot.Spec.StorageSecretName = secret.Name

						snapshot.Spec.Local = &store.LocalSpec{
							MountPath: "/repo",
							VolumeSource: core.VolumeSource{
								PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
									ClaimName: snapPVC.Name,
								},
							},
						}
					})

					AfterEach(func() {
						err := f.DeletePersistentVolumeClaim(snapPVC.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should initialize successfully", shouldInitializeFromSnapshot)
				})

				Context("From GCS backend", func() {

					BeforeEach(func() {

						skipSnapshotDataChecking = false
						secret = f.SecretForGCSBackend()
						snapshot.Spec.StorageSecretName = secret.Name
						snapshot.Spec.DatabaseName = postgres.Name

						snapshot.Spec.GCS = &store.GCSSpec{
							Bucket: os.Getenv(GCS_BUCKET_NAME),
						}
					})

					It("should run successfully", shouldInitializeFromSnapshot)
				})

			})

			// To run this test,
			// 1st: Deploy stash latest operator
			// 2nd: create postgres related tasks and functions from
			// `kubedb.dev/postgres/hack/dev/examples/stash01_config.yaml`
			Context("With Stash/Restic", func() {
				var bc *stashV1beta1.BackupConfiguration
				var bs *stashV1beta1.BackupSession
				var rs *stashV1beta1.RestoreSession
				var repo *stashV1alpha1.Repository

				BeforeEach(func() {
					skipSnapshotDataChecking = true
					if !f.FoundStashCRDs() {
						Skip("Skipping tests for stash integration. reason: stash operator is not running.")
					}
				})

				AfterEach(func() {
					By("Deleting BackupConfiguration")
					err := f.DeleteBackupConfiguration(bc.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Deleting BackupSession")
					err = f.DeleteBackupSession(bs.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Deleting RestoreSession")
					err = f.DeleteRestoreSession(rs.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Deleting Repository")
					err = f.DeleteRepository(repo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Deleting Stash RBACs")
					err = f.DeleteStashPGRBAC(postgres.ObjectMeta)
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
					By("Ensuring Stash RBACs")
					err := f.EnsureStashPGRBAC(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

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

					By("Create Repositories")
					err = f.CreateRepository(repo)
					Expect(err).NotTo(HaveOccurred())

					By("Create BackupConfiguration")
					err = f.CreateBackupConfiguration(bc)
					Expect(err).NotTo(HaveOccurred())

					By("Create BackupSession")
					err = f.CreateBackupSession(bs)
					Expect(err).NotTo(HaveOccurred())

					// eventually backupsession succeeded
					By("Check for Succeeded backupsession")
					f.EventuallyBackupSessionPhase(bs.ObjectMeta).Should(Equal(stashV1beta1.BackupSessionSucceeded))

					oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

					By("Create postgres from stash")
					*postgres = *f.Postgres()
					rs = f.RestoreSession(postgres.ObjectMeta, oldPostgres.ObjectMeta)
					postgres.Spec.DatabaseSecret = oldPostgres.Spec.DatabaseSecret
					postgres.Spec.Init = &api.InitSpec{
						StashRestoreSession: &core.LocalObjectReference{
							Name: rs.Name,
						},
					}

					// Create and wait for running Postgres
					createAndWaitForInitializing()

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
						bc = f.BackupConfiguration(postgres.ObjectMeta)
						bs = f.BackupSession(postgres.ObjectMeta)
						repo = f.Repository(postgres.ObjectMeta, secret.Name)

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

				By("Wait for postgres to be paused")
				f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

				// Create Postgres object again to resume it
				By("Create Postgres: " + postgres.Name)
				err = f.CreatePostgres(postgres)
				Expect(err).NotTo(HaveOccurred())

				By("Wait for DormantDatabase to be deleted")
				f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

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

				BeforeEach(func() {
					postgres.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/postgres-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
				})

				It("should resume DormantDatabase successfully", shouldResumeSuccessfully)
			})

			Context("With Snapshot Init", func() {

				BeforeEach(func() {
					skipSnapshotDataChecking = false
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &store.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
					snapshot.Spec.DatabaseName = postgres.Name
				})

				It("should resume successfully", func() {
					// create postgres and take snapshot
					shouldInsertDataAndTakeSnapshot()

					oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

					By("Create postgres from snapshot")
					*postgres = *f.Postgres()
					postgres.Spec.Init = &api.InitSpec{
						SnapshotSource: &api.SnapshotSourceSpec{
							Namespace: snapshot.Namespace,
							Name:      snapshot.Name,
						},
					}

					By("Creating init Snapshot Postgres without secret name" + postgres.Name)
					err = f.CreatePostgres(postgres)
					Expect(err).Should(HaveOccurred())

					// for snapshot init, user have to use older secret,
					postgres.Spec.DatabaseSecret = oldPostgres.Spec.DatabaseSecret
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Ping Database")
					f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

					By("Again delete and resume  " + postgres.Name)

					By("Delete postgres")
					err = f.DeletePostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for postgres to be paused")
					f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

					// Create Postgres object again to resume it
					By("Create Postgres: " + postgres.Name)
					err = f.CreatePostgres(postgres)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

					By("Wait for Running postgres")
					f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

					postgres, err = f.GetPostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
					Expect(postgres.Spec.Init).ShouldNot(BeNil())

					By("Ping Database")
					f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

					By("Checking postgres crd has kubedb.com/initialized annotation")
					_, err = meta_util.GetString(postgres.Annotations, api.AnnotationInitialized)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("Resume Multiple times - with init", func() {

				BeforeEach(func() {
					usedInitialized = true
					postgres.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							ScriptPath: "postgres-init-scripts/run.sh",
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/postgres-init-scripts.git",
								},
							},
						},
					}
				})

				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					for i := 0; i < 3; i++ {
						By(fmt.Sprintf("%v-th", i+1) + " time running.")
						By("Delete postgres")
						err := f.DeletePostgres(postgres.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for postgres to be paused")
						f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

						// Create Postgres object again to resume it
						By("Create Postgres: " + postgres.Name)
						err = f.CreatePostgres(postgres)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for DormantDatabase to be deleted")
						f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

						By("Wait for Running postgres")
						f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

						_, err = f.GetPostgres(postgres.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())
					}
				})
			})
		})

		Context("SnapshotScheduler", func() {

			BeforeEach(func() {
				secret = f.SecretForLocalBackend()
			})

			Context("With Startup", func() {

				BeforeEach(func() {
					postgres.Spec.BackupSchedule = &api.BackupScheduleSpec{
						CronExpression: "@every 1m",
						Backend: store.Backend{
							StorageSecretName: secret.Name,
							Local: &store.LocalSpec{
								MountPath: "/repo",
								VolumeSource: core.VolumeSource{
									EmptyDir: &core.EmptyDirVolumeSource{},
								},
							},
						},
					}
				})

				It("should run scheduler successfully", func() {
					By("Create Secret")
					err := f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Count multiple Snapshot")
					f.EventuallySnapshotCount(postgres.ObjectMeta).Should(matcher.MoreThan(3))
				})
			})

			Context("With Update", func() {
				It("should run scheduler successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Create Secret")
					err := f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					By("Update postgres")
					_, err = f.PatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
						in.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								Local: &store.LocalSpec{
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										EmptyDir: &core.EmptyDirVolumeSource{},
									},
								},
							},
						}

						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Count multiple Snapshot")
					f.EventuallySnapshotCount(postgres.ObjectMeta).Should(matcher.MoreThan(3))
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

				oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

				// -- > 1st Postgres end < --

				// -- > 2nd Postgres < --
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

				By("Deleting Postgres crd")
				err = f.DeletePostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Checking DormantDatabase is not created")
				f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

				By("Checking Wal data removed from backend")
				f.EventuallyWalDataFound(postgres).Should(BeFalse())
			}

			Context("In Local", func() {
				BeforeEach(func() {
					skipWalDataChecking = true
				})

				Context("With EmptyDir as Archive backend", func() {
					By("By definition, WAL files can not be initialized from EmptyDir. Skipping Test.")
				})

				Context("With PVC as Archive backend", func() {
					var archivePVC *core.PersistentVolumeClaim
					BeforeEach(func() {
						secret = f.SecretForLocalBackend()
						archivePVC = f.GetPersistentVolumeClaim()
						err := f.CreatePersistentVolumeClaim(archivePVC)
						Expect(err).NotTo(HaveOccurred())

						postgres.Spec.Archiver = &api.PostgresArchiverSpec{
							Storage: &store.Backend{
								Local: &store.LocalSpec{
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
											ClaimName: archivePVC.Name,
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
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
											ClaimName: archivePVC.Name,
										},
									},
								},
							},
						}
						postgres2nd.Spec.Init = &api.InitSpec{
							PostgresWAL: &api.PostgresWALSourceSpec{
								Backend: store.Backend{
									Local: &store.LocalSpec{
										MountPath: "/repo",
										SubPath:   fmt.Sprintf("%s-0", postgres.Name),
										VolumeSource: core.VolumeSource{
											PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
												ClaimName: archivePVC.Name,
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
										MountPath: "cold/sub0",
										SubPath:   fmt.Sprintf("%s-0", postgres2nd.Name),
										VolumeSource: core.VolumeSource{
											PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
												ClaimName: archivePVC.Name,
											},
										},
									},
								},
							},
						}

					})
					Context("Archive and Initialize from wal archive", func() {
						It("should archive and should resume from archive successfully", archiveAndInitializeFromLocalArchive)
					})
				})
			})

			Context("Minio S3", func() {
				BeforeEach(func() {
					skipWalDataChecking = false
					skipMinioDeployment = false
				})

				Context("With ca-cert", func() {
					BeforeEach(func() {
						By("Creating Minio server with cacert")
						addrs, err := f.CreateMinioServer(true, nil)
						Expect(err).NotTo(HaveOccurred())
						secret = f.SecretForMinioBackend()
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
						By("Creating Minio server without cacert")
						addrs, err := f.CreateMinioServer(false, nil)
						Expect(err).NotTo(HaveOccurred())
						secret = f.SecretForS3Backend()
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
				skipSnapshotDataChecking = false
				secret = f.SecretForGCSBackend()
				snapshot.Spec.StorageSecretName = secret.Name
				snapshot.Spec.GCS = &store.GCSSpec{
					Bucket: os.Getenv(GCS_BUCKET_NAME),
				}
				snapshot.Spec.DatabaseName = postgres.Name
			})

			Context("with TerminationPolicyDoNotTerminate", func() {

				BeforeEach(func() {
					skipSnapshotDataChecking = true
					postgres.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
				})

				It("should work successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Delete postgres")
					err = f.DeletePostgres(postgres.ObjectMeta)
					Expect(err).Should(HaveOccurred())

					By("Postgres is not paused. Check for postgres")
					f.EventuallyPostgres(postgres.ObjectMeta).Should(BeTrue())

					By("Check for Running postgres")
					f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

					By("Update postgres to set spec.terminationPolicy = Pause")
					_, err := f.PatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
						in.Spec.TerminationPolicy = api.TerminationPolicyPause
						return in
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("with TerminationPolicyPause (default)", func() {

				It("should create DormantDatabase and resume from it", func() {
					// Run Postgres and take snapshot
					shouldInsertDataAndTakeSnapshot()

					By("Deleting Postgres crd")
					err = f.DeletePostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// DormantDatabase.Status= paused, means postgres object is deleted
					By("Waiting for postgres to be paused")
					f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

					By("Checking PVC hasn't been deleted")
					f.EventuallyPVCCount(postgres.ObjectMeta).Should(Equal(1))

					By("Checking Secret hasn't been deleted")
					f.EventuallyDBSecretCount(postgres.ObjectMeta).Should(Equal(1))

					By("Checking snapshot hasn't been deleted")
					f.EventuallySnapshot(snapshot.ObjectMeta).Should(BeTrue())

					if !skipSnapshotDataChecking {
						By("Check for snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
					}

					// Create Postgres object again to resume it
					By("Create (resume) Postgres: " + postgres.Name)
					err = f.CreatePostgres(postgres)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

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

				AfterEach(func() {
					By("Deleting snapshot: " + snapshot.Name)
					err := f.DeleteSnapshot(snapshot.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should not create DormantDatabase and should not delete secret and snapshot", func() {
					// Run Postgres and take snapshot
					shouldInsertDataAndTakeSnapshot()

					By("Delete postgres")
					err = f.DeletePostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("wait until postgres is deleted")
					f.EventuallyPostgres(postgres.ObjectMeta).Should(BeFalse())

					By("Checking DormantDatabase is not created")
					f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

					By("Checking PVC has been deleted")
					f.EventuallyPVCCount(postgres.ObjectMeta).Should(Equal(0))

					By("Checking Secret hasn't been deleted")
					f.EventuallyDBSecretCount(postgres.ObjectMeta).Should(Equal(1))

					By("Checking Snapshot hasn't been deleted")
					f.EventuallySnapshot(snapshot.ObjectMeta).Should(BeTrue())

					if !skipSnapshotDataChecking {
						By("Check for intact snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
					}
				})
			})

			Context("with TerminationPolicyWipeOut", func() {

				BeforeEach(func() {
					postgres.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})

				It("should not create DormantDatabase and should wipeOut all", func() {
					// Run Postgres and take snapshot
					shouldInsertDataAndTakeSnapshot()

					By("Delete postgres")
					err = f.DeletePostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("wait until postgres is deleted")
					f.EventuallyPostgres(postgres.ObjectMeta).Should(BeFalse())

					By("Checking DormantDatabase is not created")
					f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

					By("Checking PVCs has been deleted")
					f.EventuallyPVCCount(postgres.ObjectMeta).Should(Equal(0))

					By("Checking Snapshots has been deleted")
					f.EventuallySnapshot(snapshot.ObjectMeta).Should(BeFalse())

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

				Context("With TerminationPolicyPause", func() {

					BeforeEach(func() {
						postgres.Spec.TerminationPolicy = api.TerminationPolicyPause
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
