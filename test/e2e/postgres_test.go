package e2e_test

import (
	"fmt"
	"os"

	core_util "github.com/appscode/kutil/core/v1"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/postgres/test/e2e/framework"
	"github.com/kubedb/postgres/test/e2e/matcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
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
		dbName                   string
		dbUser                   string
	)

	BeforeEach(func() {
		f = root.Invoke()
		postgres = f.Postgres()
		garbagePostgres = new(api.PostgresList)
		snapshot = f.Snapshot()
		secret = new(core.Secret)
		skipMessage = ""
		skipSnapshotDataChecking = true
		dbName = "postgres"
		dbUser = "postgres"
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
		if err != nil {
			if kerr.IsNotFound(err) {
				// Postgres was not created. Hence, rest of cleanup is not necessary.
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}

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

		By("Wait for postgres resources to be wipedOut")
		f.EventuallyWipedOut(postgres.ObjectMeta).Should(Succeed())
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

	Describe("Test", func() {

		Context("General", func() {

			Context("With PVC", func() {

				It("should run successfully", func() {
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
				Expect(err).Should(HaveOccurred())

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
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(1))
				})

			})

			Context("With Snapshot", func() {
				BeforeEach(func() {
					skipSnapshotDataChecking = false
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &api.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
					snapshot.Spec.DatabaseName = postgres.Name
				})

				It("should run successfully", func() {
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
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))
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
					snapshot.Spec.GCS = &api.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
					snapshot.Spec.DatabaseName = postgres.Name
				})
				It("should resume successfully", func() {
					// Create and wait for running Postgres
					createAndWaitForRunning()

					By("Creating Schema")
					f.EventuallyCreateSchema(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

					By("Creating Table")
					f.EventuallyCreateTable(postgres.ObjectMeta, dbName, dbUser, 3).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

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

					By("Ping Database")
					f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

					By("Checking Table")
					f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(3))

					Expect(postgres.Spec.Init).ShouldNot(BeNil())
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

				By("Ping Database")
				f.EventuallyPingDatabase(postgres.ObjectMeta, dbName, dbUser).Should(BeTrue())

				By("Checking Table")
				f.EventuallyCountTable(postgres.ObjectMeta, dbName, dbUser).Should(Equal(6))
			})
		})

		Context("EnvVars", func() {

			Context("With all supported EnvVars", func() {

				It("should create DB with provided EvnVars", func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}

					const (
						dataDir = "/var/pv/pgdata"
						walDir  = "/var/pv/wal"
					)
					dbName = f.App()
					dbUser = f.App()
					postgres.Spec.Env = []core.EnvVar{
						{
							Name:  PGDATA,
							Value: dataDir,
						},
						{
							Name:  POSTGRES_USER,
							Value: dbUser,
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
							Name:  POSTGRES_INITDB_XLOGDIR,
							Value: walDir,
						},
					}
					if framework.DBVersion == "10.2" {
						walEnv = []core.EnvVar{
							{
								Name:  POSTGRES_INITDB_WALDIR,
								Value: walDir,
							},
						}
					}
					postgres.Spec.Env = core_util.UpsertEnvVars(postgres.Spec.Env, walEnv...)

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
				})
			})

			Context("Root Password as EnvVar", func() {

				It("should reject to create Postgres CRD", func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}

					dbName = f.App()
					postgres.Spec.Env = []core.EnvVar{
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

				It("should reject to update EvnVar", func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}

					dbName = f.App()
					postgres.Spec.Env = []core.EnvVar{
						{
							Name:  POSTGRES_DB,
							Value: dbName,
						},
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

					By("Patching EnvVar")
					_, _, err = util.PatchPostgres(f.ExtClient(), postgres, func(in *api.Postgres) *api.Postgres {
						in.Spec.Env = []core.EnvVar{
							{
								Name:  POSTGRES_DB,
								Value: "patched-db",
							},
						}
						return in
					})
					fmt.Println(err)
					Expect(err).To(HaveOccurred())
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
					f.DeleteConfigMap(userConfig.ObjectMeta)
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
	})
})
