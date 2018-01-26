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
	)

	BeforeEach(func() {
		f = root.Invoke()
		postgres = f.Postgres()
		garbagePostgres = new(api.PostgresList)
		snapshot = f.Snapshot()
		secret = new(core.Secret)
		skipMessage = ""
		skipSnapshotDataChecking = true
	})

	var createAndWaitForRunning = func() {
		By("Create Postgres: " + postgres.Name)
		err = f.CreatePostgres(postgres)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running postgres")
		f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())
	}

	var deleteTestResource = func() {
		if postgres == nil {
			Skip("Skipping")
		}
		By("Delete postgres: " + postgres.Name)
		err = f.DeletePostgres(postgres.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for postgres to be paused")
		f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

		By("WipeOut postgres: " + postgres.Name)
		_, err := f.PatchDormantDatabase(postgres.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
			in.Spec.WipeOut = true
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Wait for postgres to be wipedOut")
		f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HaveWipedOut())

		err = f.DeleteDormantDatabase(postgres.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())
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
			f.DeleteSecret(secret.ObjectMeta)
		}
	})

	var shouldSuccessfullyRunning = func() {
		if skipMessage != "" {
			Skip(skipMessage)
		}

		// Create Postgres
		createAndWaitForRunning()
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
					if skipMessage != "" {
						Skip(skipMessage)
					}
					// Create Postgres
					createAndWaitForRunning()

					By("Creating Schema")
					f.EventuallyCreateSchema(postgres.ObjectMeta).Should(BeTrue())

					By("Creating Table")
					f.EventuallyCreateTable(postgres.ObjectMeta, 3).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta).Should(Equal(3))

					By("Delete postgres")
					err = f.DeletePostgres(postgres.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for postgres to be paused")
					f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

					_, err = f.PatchDormantDatabase(postgres.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
						in.Spec.Resume = true
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(postgres.ObjectMeta).Should(BeFalse())

					By("Wait for Running postgres")
					f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta).Should(Equal(3))
				})
			})
		})

		XContext("DoNotPause", func() {
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
				f.PatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
					in.Spec.DoNotPause = false
					return in
				})
			})
		})

		Context("Snapshot", func() {
			BeforeEach(func() {
				skipSnapshotDataChecking = false
				snapshot.Spec.DatabaseName = postgres.Name
			})

			var shouldTakeSnapshot = func() {
				// Create and wait for running Postgres
				createAndWaitForRunning()

				By("Create Secret")
				f.CreateSecret(secret)

				By("Create Snapshot")
				f.CreateSnapshot(snapshot)

				By("Check for Succeeded snapshot")
				f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

				if !skipSnapshotDataChecking {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
				}
			}

			Context("In Local", func() {
				BeforeEach(func() {
					skipSnapshotDataChecking = true
					secret = f.SecretForLocalBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Local = &api.LocalSpec{
						MountPath: "/repo",
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

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta).Should(Equal(1))
				})

			})

			Context("With Snapshot", func() {
				BeforeEach(func() {
					skipSnapshotDataChecking = false
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

					By("Creating Schema")
					f.EventuallyCreateSchema(postgres.ObjectMeta).Should(BeTrue())

					By("Creating Table")
					f.EventuallyCreateTable(postgres.ObjectMeta, 3).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta).Should(Equal(3))

					By("Create Secret")
					f.CreateSecret(secret)

					By("Create Snapshot")
					f.CreateSnapshot(snapshot)

					By("Check for Succeeded snapshot")
					f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())

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
					f.EventuallyCountTable(postgres.ObjectMeta).Should(Equal(3))
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
				f.DeletePostgres(postgres.ObjectMeta)

				By("Wait for postgres to be paused")
				f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

				_, err = f.PatchDormantDatabase(postgres.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
					in.Spec.Resume = true
					return in
				})
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
				})
				Context("with init", func() {
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
					})
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
						SnapshotStorageSpec: api.SnapshotStorageSpec{
							StorageSecretName: secret.Name,
							Local: &api.LocalSpec{
								MountPath: "/repo",
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
				})
			})

			Context("With Update", func() {
				It("should run schedular successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Create Secret")
					f.CreateSecret(secret)

					By("Update postgres")
					_, err = f.PatchPostgres(postgres.ObjectMeta, func(in *api.Postgres) *api.Postgres {
						in.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							SnapshotStorageSpec: api.SnapshotStorageSpec{
								StorageSecretName: secret.Name,
								Local: &api.LocalSpec{
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
			BeforeEach(func() {
				secret = f.SecretForS3Backend()
				postgres.Spec.Archiver = &api.PostgresArchiverSpec{
					Storage: &api.SnapshotStorageSpec{
						StorageSecretName: secret.Name,
						S3: &api.S3Spec{
							Bucket: os.Getenv(S3_BUCKET_NAME),
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

				By("Creating Schema")
				f.EventuallyCreateSchema(postgres.ObjectMeta).Should(BeTrue())

				By("Creating Table")
				f.EventuallyCreateTable(postgres.ObjectMeta, 3).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta).Should(Equal(3))

				By("Checking Archive")
				f.EventuallyCountArchive(postgres.ObjectMeta).Should(BeTrue())

				oldPostgres, err := f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

				// -- > 1st Postgres < --

				// -- > 2nd Postgres < --
				*postgres = *f.Postgres()
				postgres.Spec.Archiver = &api.PostgresArchiverSpec{
					Storage: &api.SnapshotStorageSpec{
						StorageSecretName: secret.Name,
						S3: &api.S3Spec{
							Bucket: os.Getenv(S3_BUCKET_NAME),
						},
					},
				}
				postgres.Spec.Init = &api.InitSpec{
					PostgresWAL: &api.PostgresWALSourceSpec{
						SnapshotStorageSpec: api.SnapshotStorageSpec{
							StorageSecretName: secret.Name,
							S3: &api.S3Spec{
								Bucket: os.Getenv(S3_BUCKET_NAME),
								Prefix: fmt.Sprintf("kubedb/%s/%s/archive/", postgres.Namespace, oldPostgres.Name),
							},
						},
					},
				}

				// Create Postgres
				createAndWaitForRunning()

				By("Creating Table")
				f.EventuallyCreateTable(postgres.ObjectMeta, 3).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta).Should(Equal(6))

				By("Checking Archive")
				f.EventuallyCountArchive(postgres.ObjectMeta).Should(BeTrue())

				oldPostgres, err = f.GetPostgres(postgres.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				garbagePostgres.Items = append(garbagePostgres.Items, *oldPostgres)

				// -- > 2nd Postgres < --

				// -- > 3rd Postgres < --
				*postgres = *f.Postgres()
				postgres.Spec.Init = &api.InitSpec{
					PostgresWAL: &api.PostgresWALSourceSpec{
						SnapshotStorageSpec: api.SnapshotStorageSpec{
							StorageSecretName: secret.Name,
							S3: &api.S3Spec{
								Bucket: os.Getenv(S3_BUCKET_NAME),
								Prefix: fmt.Sprintf("kubedb/%s/%s/archive/", postgres.Namespace, oldPostgres.Name),
							},
						},
					},
				}

				// Create Postgres
				createAndWaitForRunning()

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta).Should(Equal(6))
			})
		})

	})
})
