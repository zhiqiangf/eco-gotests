package tsparams

import (
	sriovv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/openshift-kni/k8sreporter"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	. "github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/ocpsriovinittools"
)

var (
	// Labels list for suite-level test selection.
	// NOTE: Only LabelSuite is included here. Individual test files add their own specific labels.
	// For example, basic.go adds LabelBasic, so filtering by "basic" will only run those 9 tests.
	Labels = []string{LabelSuite}

	// ReporterCRDsToDump tells to the reporter what CRDs to dump.
	ReporterCRDsToDump = []k8sreporter.CRData{
		{Cr: &mcfgv1.MachineConfigPoolList{}},
		{Cr: &sriovv1.SriovNetworkNodePolicyList{}},
		{Cr: &sriovv1.SriovNetworkList{}},
		{Cr: &sriovv1.SriovNetworkNodeStateList{}},
		{Cr: &sriovv1.SriovOperatorConfigList{}},
	}

	// ReporterNamespacesToDump tells to the reporter what namespaces to dump.
	ReporterNamespacesToDump = map[string]string{
		SriovOcpConfig.OcpSriovOperatorNamespace: SriovOcpConfig.OcpSriovOperatorNamespace,
		TestNamespaceName:                        "other"}
)
