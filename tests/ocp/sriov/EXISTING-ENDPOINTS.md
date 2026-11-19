# Existing VF endpoint attachment tests

This opt-in group exercises SR-IOV CNI attachment, spoof checking, trust, link
state and bidirectional IPv4 traffic using two existing, explicitly mapped
mlx5 netdevice VF resources. It supports crossed direct cables and one VF per
PF without changing firmware capacity, profiles, policies, MCP membership or
operator installation. It is an adapted attachment test, not a VF provisioning,
OVS hardware-offload, performance or multi-plane fabric qualification.

Each endpoint must already have exactly one VF, an unoccupied advertised
resource and a Succeeded NodeState. The named MCP must be stable. Release
existing workloads using those resources before the run. Select an ARM64
workload image with `/bin/sh`, `sleep`, `ping` and `ip` for ARM workers.

```bash
export ECO_OCP_SRIOV_EXISTING_ENDPOINTS='{"mcp":"arm-pool","client":{"node":"worker-a.example.test","pf":"plane0","resource":"plane_0"},"server":{"node":"worker-b.example.test","pf":"plane1","resource":"plane_1"}}'
export ECO_OCP_SRIOV_TEST_CONTAINER=<verified-workload-image-digest>
export KUBECONFIG=<existing-admin-kubeconfig-path>
ginkgo --procs=1 --label-filter=existing-endpoints ./tests/ocp/sriov
```

Use `--focus='spoof checking on'` for the first single-case smoke. Repeat
with the other physical endpoint pair to cover the other link. The environment
contains resource references only; keep kubeconfig and credentials outside the
checkout and reports.

The suite rejects a different label filter or parallel execution when the
endpoint plan is set. This prevents standard policy lifecycle groups from
running against borrowed resources. Normal suite behavior is preserved when
the variable is unset; this group skips in that case.

Cases create unique namespaces and SriovNetworks, validate the rendered NAD
resource/type, pin each pod to its explicit worker, and assert its assigned VF
PCI address. The requested attribute is read back on VF0 of each PF. Traffic
uses `ping -I net1` in both directions; absent carrier fails the case. Cleanup
releases the pods before removing only networks created by the case. Existing
policies and networks are never deleted. SR-IOV CNI does change the selected
VF's runtime attributes as part of attachment; record their prior state and
restore the original workload configuration after testing.

These cases have their own descriptive results. They do not inherit upstream
Polarion IDs or claim the original same-PF basic cases passed.
