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
		err      error
		f        *framework.Invocation
		postgres *tapi.Postgres
	)

	BeforeEach(func() {
		f = root.Invoke()
		postgres = f.Postgres()
	})

	var shouldSuccessfullyRunning = func() {
		By("Create Postgres: " + postgres.Name)
		err = f.CreatePostgres(postgres)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running postgres")
		f.EventuallyPostgresRunning(postgres.ObjectMeta).Should(BeTrue())

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

				Context("With PVC with StorageClassName", func() {
					BeforeEach(func() {
						postgres.Spec.Storage.StorageClassName = types.StringP(f.StorageClass)
					})
					It("should running successfully", shouldSuccessfullyRunning)

				})
			})
		})
	})
})
