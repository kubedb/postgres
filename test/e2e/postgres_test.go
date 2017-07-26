package e2e_test

import (
	//"github.com/appscode/go/types"
	"github.com/appscode/go/types"
	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/postgres/test/e2e/framework"
	"github.com/k8sdb/postgres/test/e2e/matcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

var _ = Describe("Postgres", func() {
	var (
		err         error
		f           *framework.Invocation
		postgres    *tapi.Postgres
		snapshot    *tapi.Snapshot
		secret      *apiv1.Secret
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

	var deleteTestResouce = func() {
		By("Delete postgres")
		err = f.DeletePostgres(postgres.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for postgres to be paused")
		f.EventuallyDormantDatabaseStatus(postgres.ObjectMeta).Should(matcher.HavePaused())

		By("WipeOut postgres")
		_, err := f.UpdateDormantDatabase(postgres.ObjectMeta, func(in tapi.DormantDatabase) tapi.DormantDatabase {
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
		deleteTestResouce()
	}

	Describe("Test", func() {

		Context("General", func() {

			It("should running successfully", shouldSuccessfullyRunning)

			Context("With PVC", func() {
				BeforeEach(func() {
					postgres.Spec.Storage = &apiv1.PersistentVolumeClaimSpec{
						Resources: apiv1.ResourceRequirements{
							Requests: apiv1.ResourceList{
								apiv1.ResourceStorage: resource.MustParse("5Gi"),
							},
						},
					}
				})
				It("should running successfully", shouldSuccessfullyRunning)

				Context("With StorageClassName", func() {
					BeforeEach(func() {
						if f.StorageClass == "" {
							skipMessage = "StorageClass is not provided"
						}
						postgres.Spec.Storage.StorageClassName = types.StringP(f.StorageClass)
					})
					It("should running successfully", shouldSuccessfullyRunning)

				})
			})
		})

		Context("DoNotPause", func() {
			BeforeEach(func() {
				postgres.Spec.DoNotPause = true
			})

			var shouldNotPause = func() {
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
				f.UpdatePostgres(postgres.ObjectMeta, func(in tapi.Postgres) tapi.Postgres {
					in.Spec.DoNotPause = false
					return in
				})

				// Delete test resource
				deleteTestResouce()
			}

			It("should work successfully", shouldNotPause)
		})

		Context("Snapshot", func() {
			BeforeEach(func() {
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
				f.EventuallySnapshotSuccessed(snapshot.ObjectMeta).Should(BeTrue())

				// Delete test resource
				deleteTestResouce()
			}

			Context("In Local", func() {
				BeforeEach(func() {
					secret = f.SecretForLocalBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Local = &tapi.LocalSpec{
						Path: "/repo",
						VolumeSource: apiv1.VolumeSource{
							EmptyDir: &apiv1.EmptyDirVolumeSource{},
						},
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In S3", func() {
				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &tapi.S3Spec{
						Bucket: "kubedb-qa",
					}
				})

				FIt("should take Snapshot successfully", shouldTakeSnapshot)
			})
		})
	})
})
