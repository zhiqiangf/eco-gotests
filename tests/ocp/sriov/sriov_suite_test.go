package sriov

import (
	"os"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/namespace"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/reportxml"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/cluster"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/params"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/reporter"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/sriovoperator"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/ocpsriovconfig"
	. "github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/ocpsriovinittools"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/tsparams"
	_ "github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/tests"
)

var (
	_, currentFile, _, _ = runtime.Caller(0)
	testNS               = namespace.NewBuilder(APIClient, tsparams.TestNamespaceName)
)

func TestLB(t *testing.T) {
	suiteConfig, _ := GinkgoConfiguration()
	if raw := os.Getenv(ocpsriovconfig.ExistingEndpointsEnv); raw != "" {
		if _, err := ocpsriovconfig.ParseExistingEndpoints(raw); err != nil {
			t.Fatal(err)
		}
		if suiteConfig.LabelFilter != ocpsriovconfig.ExistingEndpointsLabel || suiteConfig.ParallelTotal > 1 {
			t.Fatal("existing endpoint mode requires --label-filter=existing-endpoints and a single Ginkgo process")
		}
	}
	_, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = SriovOcpConfig.GetJunitReportPath(currentFile)

	RegisterFailHandler(Fail)
	RunSpecs(t, "sriov", Label(tsparams.Labels...), reporterConfig)
}

var _ = BeforeSuite(func() {
	if os.Getenv(ocpsriovconfig.ExistingEndpointsEnv) != "" {
		// The adapted group creates unique namespaces and pulls its selected
		// workload image through endpoint pods. Do not mutate the shared
		// fixed namespace or preload unrelated DPDK images in this mode.
		Expect(sriovoperator.IsSriovDeployed(APIClient, SriovOcpConfig.OcpSriovOperatorNamespace)).To(Succeed())
		return
	}
	By("Creating test namespace with privileged labels")

	for key, value := range params.PrivilegedNSLabels {
		testNS.WithLabel(key, value)
	}

	_, err := testNS.Create()
	Expect(err).ToNot(HaveOccurred(), "error to create test namespace")

	By("Verifying if sriov tests can be executed on given cluster")

	err = sriovoperator.IsSriovDeployed(APIClient, SriovOcpConfig.OcpSriovOperatorNamespace)
	Expect(err).ToNot(HaveOccurred(), "Cluster doesn't support sriov test cases")

	By("Pulling test images on cluster before running test cases")

	err = cluster.PullTestImageOnNodes(APIClient, SriovOcpConfig.WorkerLabel, SriovOcpConfig.OcpSriovTestContainer, 300)
	Expect(err).ToNot(HaveOccurred(), "Failed to pull test image on nodes")

	err = cluster.PullTestImageOnNodes(APIClient, SriovOcpConfig.WorkerLabel, SriovOcpConfig.DpdkTestContainer, 300)
	Expect(err).ToNot(HaveOccurred(), "Failed to pull DPDK test image on nodes")
})
var _ = AfterSuite(func() {
	if os.Getenv(ocpsriovconfig.ExistingEndpointsEnv) != "" {
		return
	}
	By("Deleting test namespace")

	err := testNS.DeleteAndWait(tsparams.DefaultTimeout)
	Expect(err).ToNot(HaveOccurred(), "error to delete test namespace")
})

var _ = JustAfterEach(func() {
	reporter.ReportIfFailed(
		CurrentSpecReport(), currentFile, tsparams.ReporterNamespacesToDump, tsparams.ReporterCRDsToDump)
})

var _ = ReportAfterSuite("", func(report Report) {
	reportxml.Create(report, SriovOcpConfig.GetReportPath(), SriovOcpConfig.TCPrefix)
})
