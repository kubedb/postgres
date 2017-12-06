package e2e_test

import (
	"fmt"
	"os"

	"github.com/appscode/go/types"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/postgres/test/e2e/framework"
	"github.com/kubedb/postgres/test/e2e/matcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	S3_BUCKET_NAME       = "S3_BUCKET_NAME"
	GCS_BUCKET_NAME      = "GCS_BUCKET_NAME"
	AZURE_CONTAINER_NAME = "AZURE_CONTAINER_NAME"
	SWIFT_CONTAINER_NAME = "SWIFT_CONTAINER_NAME"
	WALE_S3_PREFIX       = "WALE_S3_PREFIX"
)

var _ = Describe("Postgres", func() {
	var (
		err         error
		f           *framework.Invocation
		postgres    *api.Postgres
		snapshot    *api.Snapshot
		secret      *core.Secret
		skipMessage string
	)

	BeforeEach(func() {
		f = root.Invoke()
		postgres = f.Postgres()
		snapshot = f.Snapshot()
		skipMessage = ""
	})

	var createAndWaitForRunning = func() {
		By("Create Postgres: " + postgres.Name)
		err = f.CreatePostgres(postgres)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running postgres")
		f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())
	}

	var deleteTestResource = func() {
		By("Delete postgres")
		err = f.DeletePostgres(postgres.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for postgres to be paused")
		f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

		By("WipeOut postgres")
		_, err := f.TryPatchDormantDatabase(postgres.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
			in.Spec.WipeOut = true
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Wait for postgres to be wipedOut")
		f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HaveWipedOut())

		err = f.DeleteDormantDatabase(postgres.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())
	}

	var shouldSuccessfullyRunning = func() {
		if skipMessage != "" {
			Skip(skipMessage)
		}

		// Create Postgres
		createAndWaitForRunning()

		// Delete test resource
		deleteTestResource()
	}

	Describe("Test", func() {

		Context("General", func() {

			Context("-", func() {
				It("should run successfully", shouldSuccessfullyRunning)
			})

			Context("With PVC", func() {
				BeforeEach(func() {
					if f.StorageClass == "" {
						skipMessage = "Missing StorageClassName. Provide as flag to test this."
					}
					postgres.Spec.Storage = &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse("5Gi"),
							},
						},
						StorageClassName: types.StringP(f.StorageClass),
					}
				})
				It("should run successfully", func() {
					// Create Postgres
					createAndWaitForRunning()

					By("Check for Postgres client")
					f.EventuallyPostgresClientReady(postgres.ObjectMeta).Should(BeTrue())

					pgClient, err := f.GetPostgresClient(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					err = f.CreateSchema(pgClient)
					Expect(err).NotTo(HaveOccurred())

					By("Creating Table")
					err = f.CreateTable(pgClient, 3)
					Expect(err).NotTo(HaveOccurred())

					By("Checking Table")
					f.EventuallyPostgresTableCount(pgClient).Should(Equal(3))

					By("Update postgres to set Replicas=0")
					pg, err := f.TryPatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
						in.Spec.Replicas = 0
						return in
					})
					*postgres = *pg

					By("Counting for Postgres Pod")
					f.EventuallyPostgresPodCount(postgres.ObjectMeta).Should(BeZero())

					By("Update postgres to set Replicas=1")
					pg, err = f.TryPatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
						in.Spec.Replicas = 1
						return in
					})
					*postgres = *pg

					By("Counting for Postgres Pod")
					f.EventuallyPostgresPodCount(postgres.ObjectMeta).Should(BeNumerically("==", 1))

					By("Check for Postgres client")
					f.EventuallyPostgresClientReady(postgres.ObjectMeta).Should(BeTrue())

					pgClient, err = f.GetPostgresClient(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Checking Table")
					f.EventuallyPostgresTableCount(pgClient).Should(Equal(3))

					// Delete test resource
					deleteTestResource()
				})
			})
		})

		Context("DoNotPause", func() {
			BeforeEach(func() {
				postgres.Spec.DoNotPause = true
			})

			It("should work successfully", func() {
				// Create and wait for running Postgres
				createAndWaitForRunning()

				By("Delete postgres")
				err = f.DeletePostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Postgres is not paused. Check for postgres")
				f.EventuallyPostgres(postgres.ObjectMeta).Should(BeTrue())

				By("Check for Running postgres")
				f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

				By("Update postgres to set DoNotPause=false")
				f.TryPatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
					in.Spec.DoNotPause = false
					return in
				})

				// Delete test resource
				deleteTestResource()
			})
		})

		Context("Snapshot", func() {
			var skipDataCheck bool

			AfterEach(func() {
				f.DeleteSecret(secret.ObjectMeta)
			})

			BeforeEach(func() {
				skipDataCheck = false
				snapshot.Spec.DatabaseName = postgres.Name
			})

			var shouldTakeSnapshot = func() {
				// Create and wait for running Postgres
				createAndWaitForRunning()

				By("Create Secret")
				f.CreateSecret(secret)

				By("Create Snapshot")
				f.CreateSnapshot(snapshot)

				By("Check for Successed snapshot")
				f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSuccessed))

				if !skipDataCheck {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
				}

				// Delete test resource
				deleteTestResource()

				if !skipDataCheck {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
				}
			}

			Context("In Local", func() {
				BeforeEach(func() {
					skipDataCheck = true
					secret = f.SecretForLocalBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Local = &api.LocalSpec{
						Path: "/repo",
						VolumeSource: core.VolumeSource{
							EmptyDir: &core.EmptyDirVolumeSource{},
						},
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In S3", func() {
				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &api.S3Spec{
						Bucket: os.Getenv(S3_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In GCS", func() {
				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &api.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In Azure", func() {
				BeforeEach(func() {
					secret = f.SecretForAzureBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Azure = &api.AzureSpec{
						Container: os.Getenv(AZURE_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In Swift", func() {
				BeforeEach(func() {
					secret = f.SecretForSwiftBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Swift = &api.SwiftSpec{
						Container: os.Getenv(SWIFT_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
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

					By("Check for Postgres client")
					f.EventuallyPostgresClientReady(postgres.ObjectMeta).Should(BeTrue())

					pgClient, err := f.GetPostgresClient(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Checking Table")
					f.EventuallyPostgresTableCount(pgClient).Should(Equal(1))

					// Delete test resource
					deleteTestResource()
				})

			})

			Context("With Snapshot", func() {
				AfterEach(func() {
					f.DeleteSecret(secret.ObjectMeta)
				})

				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &api.S3Spec{
						Bucket: os.Getenv(S3_BUCKET_NAME),
					}
					snapshot.Spec.DatabaseName = postgres.Name
				})

				It("should run successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Check for Postgres client")
					f.EventuallyPostgresClientReady(postgres.ObjectMeta).Should(BeTrue())

					pgClient, err := f.GetPostgresClient(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					err = f.CreateSchema(pgClient)
					Expect(err).NotTo(HaveOccurred())

					By("Creating Table")
					err = f.CreateTable(pgClient, 3)
					Expect(err).NotTo(HaveOccurred())

					By("Checking Table")
					f.EventuallyPostgresTableCount(pgClient).Should(Equal(3))

					By("Create Secret")
					f.CreateSecret(secret)

					By("Create Snapshot")
					f.CreateSnapshot(snapshot)

					By("Check for Successed snapshot")
					f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSuccessed))

					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())

					oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

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

					By("Check for Postgres client")
					f.EventuallyPostgresClientReady(postgres.ObjectMeta).Should(BeTrue())

					pgClient, err = f.GetPostgresClient(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Checking Table")
					f.EventuallyPostgresTableCount(pgClient).Should(Equal(3))

					// Delete test resource
					deleteTestResource()
					*postgres = *oldPostgres
					// Delete test resource
					deleteTestResource()
				})
			})
		})

		Context("Resume", func() {
			var usedInitSpec bool
			BeforeEach(func() {
				usedInitSpec = false
			})

			var shouldResumeSuccessfully = func() {
				// Create and wait for running Postgres
				createAndWaitForRunning()

				By("Delete postgres")
				f.DeletePostgres(postgres.ObjectMeta)

				By("Wait for postgres to be paused")
				f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

				_, err = f.TryPatchDormantDatabase(postgres.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
					in.Spec.Resume = true
					return in
				})
				Expect(err).NotTo(HaveOccurred())

				By("Wait for DormantDatabase to be deleted")
				f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

				By("Wait for Running postgres")
				f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

				_postgres, err := f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				*postgres = *_postgres
				if usedInitSpec {
					Expect(postgres.Spec.Init).Should(BeNil())
					Expect(postgres.Annotations[api.PostgresInitSpec]).ShouldNot(BeEmpty())
				}

				// Delete test resource
				deleteTestResource()
			}

			Context("-", func() {
				It("should resume DormantDatabase successfully", shouldResumeSuccessfully)
			})

			Context("With Init", func() {
				BeforeEach(func() {
					usedInitSpec = true
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

			Context("With original Postgres", func() {
				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Delete postgres")
					f.DeletePostgres(postgres.ObjectMeta)

					By("Wait for postgres to be paused")
					f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

					// Create Postgres object again to resume it
					By("Create Postgres: " + postgres.Name)
					err = f.CreatePostgres(postgres)
					if err != nil {
						fmt.Println(err)
					}
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

					By("Wait for Running postgres")
					f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

					postgres, err = f.GetPostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// Delete test resource
					deleteTestResource()
				})
				Context("with init", func() {
					BeforeEach(func() {
						usedInitSpec = true
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
							f.DeletePostgres(postgres.ObjectMeta)

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

							_, err := f.GetPostgres(postgres.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						}

						// Delete test resource
						deleteTestResource()
					})
				})
			})
		})

		Context("SnapshotScheduler", func() {
			AfterEach(func() {
				f.DeleteSecret(secret.ObjectMeta)
			})

			BeforeEach(func() {
				secret = f.SecretForLocalBackend()
			})

			Context("With Startup", func() {
				BeforeEach(func() {
					postgres.Spec.BackupSchedule = &api.BackupScheduleSpec{
						CronExpression: "@every 1m",
						SnapshotStorageSpec: api.SnapshotStorageSpec{
							StorageSecretName: secret.Name,
							Local: &api.LocalSpec{
								Path: "/repo",
								VolumeSource: core.VolumeSource{
									EmptyDir: &core.EmptyDirVolumeSource{},
								},
							},
						},
					}
				})

				It("should run schedular successfully", func() {
					By("Create Secret")
					f.CreateSecret(secret)

					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Count multiple Snapshot")
					f.EventuallySnapshotCount(postgres.ObjectMeta).Should(matcher.MoreThan(3))

					deleteTestResource()
				})
			})

			Context("With Update", func() {
				It("should run schedular successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Create Secret")
					f.CreateSecret(secret)

					By("Update postgres")
					_, err = f.TryPatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
						in.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							SnapshotStorageSpec: api.SnapshotStorageSpec{
								StorageSecretName: secret.Name,
								Local: &api.LocalSpec{
									Path: "/repo",
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

					deleteTestResource()
				})
			})
		})

		Context("Archive with wal-g", func() {
			BeforeEach(func() {
				secret = f.SecretForS3Backend()
				postgres.Spec.Archiver = &api.PostgresArchiverSpec{
					Storage: &api.SnapshotStorageSpec{
						StorageSecretName: secret.Name,
						S3: &api.S3Spec{
							Bucket: os.Getenv(S3_BUCKET_NAME),
							Prefix: postgres.Name,
						},
					},
				}
			})

			It("should archive successfully", func() {
				// -- > 1st Postgres < --
				err := f.CreateSecret(secret)
				Expect(err).NotTo(HaveOccurred())

				// Create Postgres
				createAndWaitForRunning()

				By("Check for Postgres client")
				f.EventuallyPostgresClientReady(postgres.ObjectMeta).Should(BeTrue())

				pgClient, err := f.GetPostgresClient(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				err = f.CreateSchema(pgClient)
				Expect(err).NotTo(HaveOccurred())

				By("Creating Table")
				err = f.CreateTable(pgClient, 3)
				Expect(err).NotTo(HaveOccurred())

				By("Checking Table")
				f.EventuallyPostgresTableCount(pgClient).Should(Equal(3))

				By("Count Archive")
				count, err := f.CountArchive(pgClient)
				Expect(err).NotTo(HaveOccurred())

				By("Checking Archive")
				f.EventuallyPostgresArchiveCount(pgClient).Should(BeNumerically(">", count))

				oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				// Delete test resource
				deleteTestResource()
				// -- > 1st Postgres < --

				// -- > 2nd Postgres < --
				*postgres = *f.Postgres()
				postgres.Spec.Archiver = &api.PostgresArchiverSpec{
					Storage: &api.SnapshotStorageSpec{
						StorageSecretName: secret.Name,
						S3: &api.S3Spec{
							Bucket: os.Getenv(S3_BUCKET_NAME),
							Prefix: postgres.Name,
						},
					},
				}
				postgres.Spec.Init = &api.InitSpec{
					PostgresWAL: &api.PostgresWALSourceSpec{
						SnapshotStorageSpec: api.SnapshotStorageSpec{
							StorageSecretName: secret.Name,
							S3: &api.S3Spec{
								Bucket: os.Getenv(S3_BUCKET_NAME),
								Prefix: oldPostgres.Name,
							},
						},
					},
				}

				// Create Postgres
				createAndWaitForRunning()

				By("Check for Postgres client")
				f.EventuallyPostgresClientReady(postgres.ObjectMeta).Should(BeTrue())

				pgClient, err = f.GetPostgresClient(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Creating Table")
				err = f.CreateTable(pgClient, 3)
				Expect(err).NotTo(HaveOccurred())

				By("Checking Table")
				f.EventuallyPostgresTableCount(pgClient).Should(Equal(6))

				By("Count Archive")
				count, err = f.CountArchive(pgClient)
				Expect(err).NotTo(HaveOccurred())

				By("Checking Archive")
				f.EventuallyPostgresArchiveCount(pgClient).Should(BeNumerically(">", count))

				oldPostgres, err = f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				// Delete test resource
				deleteTestResource()
				// -- > 2nd Postgres < --

				// -- > 3rd Postgres < --
				*postgres = *f.Postgres()
				postgres.Spec.Init = &api.InitSpec{
					PostgresWAL: &api.PostgresWALSourceSpec{
						SnapshotStorageSpec: api.SnapshotStorageSpec{
							StorageSecretName: secret.Name,
							S3: &api.S3Spec{
								Bucket: os.Getenv(S3_BUCKET_NAME),
								Prefix: oldPostgres.Name,
							},
						},
					},
				}

				// Create Postgres
				createAndWaitForRunning()

				By("Check for Postgres client")
				f.EventuallyPostgresClientReady(postgres.ObjectMeta).Should(BeTrue())

				pgClient, err = f.GetPostgresClient(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("Checking Table")
				f.EventuallyPostgresTableCount(pgClient).Should(Equal(6))

				// Delete test resource
				deleteTestResource()
			})

		})

	})
})
