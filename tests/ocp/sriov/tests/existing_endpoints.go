package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	sriovv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/namespace"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/pod"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/sriov"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/cluster"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/params"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/ocpsriovconfig"
	. "github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/ocpsriovinittools"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/ocp/sriov/internal/sriovenv"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var schemaNAD = schema.GroupVersionResource{Group: "k8s.cni.cncf.io", Version: "v1", Resource: "network-attachment-definitions"}

type existingVF struct {
	Endpoint ocpsriovconfig.ExistingEndpoint
	PCI      string
}

type existingCase struct {
	Attribute string
	Value     string
}

// This is deliberately a separate adapted test group: it does not claim VF
// provisioning coverage and never creates, deletes or updates a NodePolicy.
var _ = Describe("SR-IOV existing endpoint attachment", Ordered,
	Label(ocpsriovconfig.ExistingEndpointsLabel), ContinueOnFailure, func() {
		var plan *ocpsriovconfig.ExistingEndpoints

		BeforeAll(func() {
			raw := os.Getenv(ocpsriovconfig.ExistingEndpointsEnv)
			if raw == "" {
				Skip("existing endpoint plan was not supplied")
			}
			var err error
			plan, err = ocpsriovconfig.ParseExistingEndpoints(raw)
			Expect(err).NotTo(HaveOccurred())
			Expect(APIClient.AttachScheme(sriovv1.AddToScheme)).To(Succeed())
			Expect(cluster.WaitForMcpStable(APIClient, 5*time.Minute, 10*time.Second, plan.MCP)).To(Succeed())
			AddReportEntry("coverage", "adapted existing-VF attachment, attributes and traffic; no VF provisioning, HWOL or fabric qualification")
		})

		DescribeTable("VF attributes and bidirectional traffic", func(test existingCase) {
			endpoints := []existingVF{inspectExistingEndpoint(plan.Client), inspectExistingEndpoint(plan.Server)}
			nsName := "sriov-existing-" + uuid.NewString()[:8]
			ns := namespace.NewBuilder(APIClient, nsName).WithMultipleLabels(params.PrivilegedNSLabels).
				WithLabel("eco-gotests.openshift.io/test", "sriov-existing-endpoints")
			// An atomic create rejects collisions instead of adopting an
			// existing namespace through the idempotent builder Create method.
			err := APIClient.Create(context.Background(), ns.Definition)
			Expect(err).NotTo(HaveOccurred())
			var networks []*sriov.NetworkBuilder
			DeferCleanup(func() error {
				// Release all VFs before deleting networks. Delete only resources
				// successfully created by this case in its unique namespace.
				if err := ns.DeleteAndWait(2 * time.Minute); err != nil {
					return err
				}
				var errs []error
				for _, network := range networks {
					errs = append(errs, network.DeleteAndWait(time.Minute))
				}
				return errors.Join(errs...)
			})
			workloads := make([]*pod.Builder, 0, 2)
			addresses := []string{"192.0.2.1/30", "192.0.2.2/30"}
			for index, endpoint := range endpoints {
				name := fmt.Sprintf("%s-%d", nsName, index)
				network := sriov.NewNetworkBuilder(APIClient, name,
					SriovOcpConfig.OcpSriovOperatorNamespace, nsName, endpoint.Endpoint.Resource).
					WithStaticIpam().WithIPAddressSupport().WithMacAddressSupport()
				switch test.Attribute {
				case "spoofchk":
					network = network.WithSpoof(test.Value == "on")
				case "trust":
					network = network.WithTrustFlag(test.Value == "on")
				case "link-state":
					network = network.WithLinkState(test.Value)
				}
				created, err := network.Create()
				Expect(err).NotTo(HaveOccurred())
				networks = append(networks, created)
				// Installed partner operators need not implement newer network
				// status conditions. Verify the actual rendered NAD instead.
				Eventually(func(g Gomega) {
					var attachment struct {
						Spec struct {
							Config string `json:"config"`
						} `json:"spec"`
						Metadata struct {
							Annotations map[string]string `json:"annotations"`
						} `json:"metadata"`
					}
					data, err := APIClient.Interface.Resource(schemaNAD).Namespace(nsName).Get(context.Background(), name, metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					serialized, err := data.MarshalJSON()
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(json.Unmarshal(serialized, &attachment)).To(Succeed())
					g.Expect(attachment.Metadata.Annotations["k8s.v1.cni.cncf.io/resourceName"]).To(Equal("openshift.io/" + endpoint.Endpoint.Resource))
					var config map[string]any
					g.Expect(json.Unmarshal([]byte(attachment.Spec.Config), &config)).To(Succeed())
					g.Expect(config["type"]).To(Equal("sriov"))
				}).WithTimeout(time.Minute).WithPolling(2 * time.Second).Should(Succeed())
				selection := pod.StaticIPAnnotationWithMacAddress(name, []string{addresses[index]}, fmt.Sprintf("02:00:00:00:23:%02x", index+1))
				builder := pod.NewBuilder(APIClient, fmt.Sprintf("endpoint-%d", index), nsName, SriovOcpConfig.OcpSriovTestContainer).
					WithPrivilegedFlag().WithNodeSelector(map[string]string{"kubernetes.io/hostname": endpoint.Endpoint.Node}).
					WithCommand([]string{"/bin/sh", "-c", "exec sleep infinity"}).WithSecondaryNetwork(selection)
				workload, err := builder.Create()
				Expect(err).NotTo(HaveOccurred())
				workloads = append(workloads, workload)
				Expect(workload.WaitUntilReady(3 * time.Minute)).To(Succeed())
				pci, err := sriovenv.GetPciAddress(nsName, workload.Definition.Name, "net1")
				Expect(err).NotTo(HaveOccurred())
				Expect(pci).To(Equal(endpoint.PCI), "pod must receive the VF on the explicit physical endpoint")
				Expect(sriovenv.VerifyInterfaceReady(workload, "net1")).To(Succeed())
				carrier, err := sriovenv.CheckInterfaceCarrier(workload, "net1")
				Expect(err).NotTo(HaveOccurred())
				Expect(carrier).To(BeTrue(), "missing carrier is a failure, not a passed or skipped traffic case")
				verifyExistingVFAttribute(endpoint.Endpoint, test)
			}
			for index, workload := range workloads {
				destination := strings.Split(addresses[1-index], "/")[0]
				output, err := workload.ExecCommand([]string{"ping", "-I", "net1", "-c", "3", "-W", "2", "-w", "10", destination})
				AddReportEntry(fmt.Sprintf("endpoint-%d-ping", index), output.String())
				Expect(err).NotTo(HaveOccurred(), "secondary-interface traffic must pass in both directions")
			}
		},
			Entry("spoof checking on", existingCase{"spoofchk", "on"}),
			Entry("spoof checking off", existingCase{"spoofchk", "off"}),
			Entry("trust off", existingCase{"trust", "off"}),
			Entry("trust on", existingCase{"trust", "on"}),
			Entry("link state auto", existingCase{"link-state", "auto"}),
			Entry("link state enable", existingCase{"link-state", "enable"}),
		)
	})

func inspectExistingEndpoint(endpoint ocpsriovconfig.ExistingEndpoint) existingVF {
	var node corev1.Node
	Expect(APIClient.Get(context.Background(), types.NamespacedName{Name: endpoint.Node}, &node)).To(Succeed())
	Expect(node.Spec.Unschedulable).To(BeFalse())
	ready := false
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			ready = true
		}
	}
	Expect(ready).To(BeTrue(), "endpoint node must be Ready")
	var consumers corev1.PodList
	Expect(APIClient.List(context.Background(), &consumers)).To(Succeed())
	for _, consumer := range consumers.Items {
		if consumer.Spec.NodeName != endpoint.Node || consumer.Status.Phase == corev1.PodSucceeded || consumer.Status.Phase == corev1.PodFailed {
			continue
		}
		for _, container := range consumer.Spec.Containers {
			quantity := container.Resources.Requests[corev1.ResourceName("openshift.io/"+endpoint.Resource)]
			Expect(quantity.IsZero()).To(BeTrue(), "release existing VF consumer %s/%s before testing", consumer.Namespace, consumer.Name)
		}
	}
	var state sriovv1.SriovNetworkNodeState
	Expect(APIClient.Get(context.Background(), types.NamespacedName{Name: endpoint.Node,
		Namespace: SriovOcpConfig.OcpSriovOperatorNamespace}, &state)).To(Succeed())
	Expect(state.Status.SyncStatus).To(Equal("Succeeded"))
	matchedGroup := false
	for _, intf := range state.Spec.Interfaces {
		if intf.Name == endpoint.PF {
			for _, group := range intf.VfGroups {
				if group.ResourceName == endpoint.Resource && group.DeviceType == "netdevice" && group.VfRange == "0-0" {
					matchedGroup = true
				}
			}
		}
	}
	Expect(matchedGroup).To(BeTrue(), "explicit PF must advertise the existing one-VF resource")
	for _, intf := range state.Status.Interfaces {
		if intf.Name == endpoint.PF {
			Expect(intf.NumVfs).To(Equal(1))
			Expect(intf.VFs).To(HaveLen(1))
			Expect(intf.VFs[0].Driver).To(Equal("mlx5_core"))
			Expect(intf.VFs[0].PciAddress).NotTo(BeEmpty())
			return existingVF{endpoint, intf.VFs[0].PciAddress}
		}
	}
	Fail("explicit PF missing from NodeState inventory")
	return existingVF{}
}

func verifyExistingVFAttribute(endpoint ocpsriovconfig.ExistingEndpoint, test existingCase) {
	// PF names are validated before use in this host read-only command.
	output, err := cluster.ExecCmdWithStdout(APIClient, "ip -j link show dev "+endpoint.PF,
		metav1.ListOptions{LabelSelector: "kubernetes.io/hostname=" + endpoint.Node})
	Expect(err).NotTo(HaveOccurred())
	var links []struct {
		VFs []map[string]any `json:"vfinfo_list"`
	}
	Expect(json.Unmarshal([]byte(output[endpoint.Node]), &links)).To(Succeed())
	Expect(links).To(HaveLen(1))
	Expect(links[0].VFs).To(HaveLen(1))
	vf := links[0].VFs[0]
	Expect(vf["vf"]).To(Equal(float64(0)))
	if test.Attribute == "link-state" {
		Expect(vf["link_state"]).To(Equal(test.Value))
	} else {
		Expect(vf[test.Attribute]).To(Equal(test.Value == "on"))
	}
	AddReportEntry(endpoint.Node+"-vf-state", output[endpoint.Node])
}
