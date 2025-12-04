package sriovenv

import (
	"fmt"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/clusterversion"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/olm"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/pod"
	. "github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/ocpsriovinittools"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// GetOCPVersion retrieves the OpenShift cluster version.
func GetOCPVersion() (string, error) {
	klog.V(90).Info("Retrieving OpenShift cluster version")

	clusterVersion, err := clusterversion.Pull(APIClient)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster version: %w", err)
	}

	if clusterVersion.Object == nil {
		return "", fmt.Errorf("cluster version object is nil")
	}

	// Try to get completed version from history first
	histories := clusterVersion.Object.Status.History
	for i := 0; i < len(histories); i++ {
		if histories[i].State == configv1.CompletedUpdate {
			version := histories[i].Version
			klog.V(90).Infof("Found completed cluster version: %s", version)

			return version, nil
		}
	}

	// Fall back to desired version
	version := clusterVersion.Object.Status.Desired.Version
	klog.V(90).Infof("Using desired cluster version: %s", version)

	return version, nil
}

// GetSriovOperatorVersion retrieves the SR-IOV operator version from CSV.
func GetSriovOperatorVersion() (string, error) {
	namespace := SriovOcpConfig.OcpSriovOperatorNamespace
	klog.V(90).Infof("Retrieving SR-IOV operator version from namespace %s", namespace)

	// List CSVs in the SR-IOV operator namespace
	csvList, err := olm.ListClusterServiceVersion(APIClient, namespace)
	if err != nil {
		return "", fmt.Errorf("failed to list CSVs in namespace %s: %w", namespace, err)
	}

	// Look for SR-IOV operator CSV
	const expectedCSVNameSubstring = "sriov-network-operator"

	for _, csv := range csvList {
		csvName := strings.ToLower(csv.Object.Name)
		csvDisplayName := strings.ToLower(csv.Object.Spec.DisplayName)

		if strings.Contains(csvName, expectedCSVNameSubstring) ||
			strings.Contains(csvDisplayName, expectedCSVNameSubstring) ||
			strings.Contains(csvName, "sriov") ||
			strings.Contains(csvDisplayName, "sriov") {
			version := csv.Object.Spec.Version.String()
			klog.V(90).Infof("Found SR-IOV operator CSV: %s, Version: %s", csv.Object.Name, version)

			return version, nil
		}
	}

	return "", fmt.Errorf("SR-IOV operator CSV not found in namespace %s", namespace)
}

// PodContainerInfo represents container information from a pod.
type PodContainerInfo struct {
	PodName       string
	ContainerName string
	Image         string
}

// GetSriovOperatorPodContainers retrieves container information from all pods in the SR-IOV operator namespace.
func GetSriovOperatorPodContainers() ([]PodContainerInfo, error) {
	namespace := SriovOcpConfig.OcpSriovOperatorNamespace
	klog.V(90).Infof("Retrieving SR-IOV operator pod container information from namespace %s", namespace)

	podList, err := pod.List(APIClient, namespace, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}

	var containerInfo []PodContainerInfo

	for _, podBuilder := range podList {
		if podBuilder.Object == nil {
			continue
		}

		podName := podBuilder.Object.Name

		// Get init containers
		for _, container := range podBuilder.Object.Spec.InitContainers {
			containerInfo = append(containerInfo, PodContainerInfo{
				PodName:       podName,
				ContainerName: container.Name + " (init)",
				Image:         container.Image,
			})
		}

		// Get regular containers
		for _, container := range podBuilder.Object.Spec.Containers {
			containerInfo = append(containerInfo, PodContainerInfo{
				PodName:       podName,
				ContainerName: container.Name,
				Image:         container.Image,
			})
		}
	}

	if len(containerInfo) == 0 {
		return nil, fmt.Errorf("no containers found in pods in namespace %s", namespace)
	}

	return containerInfo, nil
}
