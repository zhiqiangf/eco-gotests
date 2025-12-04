package sriov

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/namespace"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/reportxml"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/params"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/reporter"
	. "github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/ocpsriovinittools"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/sriovenv"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/tsparams"
	_ "github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/tests"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
)

var _, currentFile, _, _ = runtime.Caller(0)

var (
	testNS *namespace.Builder
)

func TestSriov(t *testing.T) {
	_, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = SriovOcpConfig.GetJunitReportPath(currentFile)

	RegisterFailHandler(Fail)
	RunSpecs(t, "OCP SR-IOV Suite", Label(tsparams.Labels...), reporterConfig)
}

var _ = BeforeSuite(func() {
	By("Validating test configuration")
	// WorkerLabel is provided by GeneralConfig and is always in correct format
	if SriovOcpConfig.WorkerLabel == "" {
		Fail("Worker label not configured. Ensure ECO_WORKER_LABEL is set or default config is loaded.")
	}

	By("Cleaning up leftover resources from previous test runs")
	err := sriovenv.CleanupLeftoverResources()
	Expect(err).ToNot(HaveOccurred(), "Failed to cleanup leftover resources")

	By("Creating test namespace with privileged labels")
	testNS = namespace.NewBuilder(APIClient, tsparams.TestNamespaceName)
	for key, value := range params.PrivilegedNSLabels {
		testNS.WithLabel(key, value)
	}

	_, err = testNS.Create()
	if err != nil {
		// Handle pre-existing namespace from previous partial failures
		if apierrors.IsAlreadyExists(err) {
			klog.V(90).Infof("Test namespace %q already exists, deleting and recreating", tsparams.TestNamespaceName)

			existingNS, pullErr := namespace.Pull(APIClient, tsparams.TestNamespaceName)
			Expect(pullErr).ToNot(HaveOccurred(), "Failed to pull existing test namespace")

			deleteErr := existingNS.DeleteAndWait(tsparams.DefaultTimeout)
			Expect(deleteErr).ToNot(HaveOccurred(), "Failed to delete existing test namespace")

			testNS = namespace.NewBuilder(APIClient, tsparams.TestNamespaceName)
			for key, value := range params.PrivilegedNSLabels {
				testNS.WithLabel(key, value)
			}

			_, err = testNS.Create()
			Expect(err).ToNot(HaveOccurred(), "Failed to recreate test namespace")
		} else {
			Fail(fmt.Sprintf("Failed to create test namespace: %v", err))
		}
	}

	By("Verifying if sriov tests can be executed on given cluster")
	err = sriovenv.CheckSriovOperatorStatus()
	Expect(err).ToNot(HaveOccurred(), "Cluster doesn't support sriov test cases")
})

var _ = AfterSuite(func() {
	By("Deleting test namespace")
	if testNS != nil {
		err := testNS.DeleteAndWait(tsparams.DefaultTimeout)
		Expect(err).ToNot(HaveOccurred(), "Failed to delete test namespace")
	}
})

var _ = JustAfterEach(func() {
	reporter.ReportIfFailed(
		CurrentSpecReport(),
		currentFile,
		tsparams.ReporterNamespacesToDump,
		tsparams.ReporterCRDsToDump)
})

var _ = ReportAfterSuite("", func(report Report) {
	// Get cluster and operator versions for report metadata
	var ocpVersion, sriovVersion string
	var versionErr error

	ocpVersion, versionErr = sriovenv.GetOCPVersion()
	if versionErr != nil {
		klog.V(90).Infof("Failed to get OCP version: %v", versionErr)
		ocpVersion = "unknown"
	}

	sriovVersion, versionErr = sriovenv.GetSriovOperatorVersion()
	if versionErr != nil {
		klog.V(90).Infof("Failed to get SR-IOV operator version: %v", versionErr)
		sriovVersion = "unknown"
	}

	// Get SR-IOV operator pod container information
	var containerInfo []sriovenv.PodContainerInfo

	containerInfo, versionErr = sriovenv.GetSriovOperatorPodContainers()
	if versionErr != nil {
		klog.V(90).Infof("Failed to get SR-IOV operator pod containers: %v", versionErr)
		containerInfo = []sriovenv.PodContainerInfo{}
	}

	// Log version information
	klog.V(90).Infof("Test Report Metadata - OCP Version: %s, SR-IOV Operator Version: %s", ocpVersion, sriovVersion)

	// Create the report
	reportPath := SriovOcpConfig.GetReportPath()
	reportxml.Create(report, reportPath, SriovOcpConfig.TCPrefix)

	// Write version metadata to a separate file
	if reportPath != "" {
		metadataPath := strings.TrimSuffix(reportPath, ".xml") + "_metadata.txt"

		var metadataBuilder strings.Builder
		metadataBuilder.WriteString(fmt.Sprintf("OpenShift Cluster Version: %s\n", ocpVersion))
		metadataBuilder.WriteString(fmt.Sprintf("SR-IOV Operator Version: %s\n", sriovVersion))
		metadataBuilder.WriteString(fmt.Sprintf("Report Generated: %s\n", report.EndTime.Format("2006-01-02 15:04:05")))
		metadataBuilder.WriteString("\nSR-IOV Operator Pod Containers:\n")

		if len(containerInfo) > 0 {
			// Group by pod name for better readability
			podMap := make(map[string][]sriovenv.PodContainerInfo)
			for _, info := range containerInfo {
				podMap[info.PodName] = append(podMap[info.PodName], info)
			}

			podNames := make([]string, 0, len(podMap))
			for podName := range podMap {
				podNames = append(podNames, podName)
			}

			sort.Strings(podNames)

			for _, podName := range podNames {
				containers := podMap[podName]
				metadataBuilder.WriteString(fmt.Sprintf("\n  Pod: %s\n", podName))

				for _, container := range containers {
					metadataBuilder.WriteString(fmt.Sprintf("    Container: %s\n", container.ContainerName))
					metadataBuilder.WriteString(fmt.Sprintf("    Image: %s\n", container.Image))
				}
			}
		} else {
			metadataBuilder.WriteString("  (No container information available)\n")
		}

		if err := os.WriteFile(metadataPath, []byte(metadataBuilder.String()), 0600); err != nil {
			klog.Errorf("Failed to write metadata file %s: %v", metadataPath, err)
		} else {
			klog.V(90).Infof("Version metadata written to: %s", metadataPath)
		}
	}

	// Print report file locations
	junitReportPath := SriovOcpConfig.GetJunitReportPath(currentFile)
	fmt.Printf("\n=== Test Report Files ===\n")
	fmt.Printf("JUnit Report: %s\n", junitReportPath)

	if reportPath != "" {
		fmt.Printf("Test Run Report: %s\n", reportPath)
		fmt.Printf("Metadata File: %s\n", strings.TrimSuffix(reportPath, ".xml")+"_metadata.txt")
	} else {
		fmt.Printf("Test Run Report: (disabled - set ECO_ENABLE_REPORT=true to enable)\n")
		fmt.Printf("Metadata File: (disabled - set ECO_ENABLE_REPORT=true to enable)\n")
	}

	fmt.Printf("========================\n\n")
})
