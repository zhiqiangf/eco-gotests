# VF Resource Availability - Diagnosis Checklist

Use this checklist to diagnose why pods are failing to schedule due to insufficient VF resources.

## 🔍 Quick Diagnosis (Run These First)

### 1. Check SRIOV Operator Status
```bash
oc get pods -n openshift-sriov-network-operator
```
**Expected Output:**
- `sriov-network-config-daemon` - Running (multiple pods, one per node)
- `sriov-network-operator` - Running  
- `sriov-network-webhook` - Running (if version 4.12+)
- `network-resources-injector` - Running

**If any pod is NOT Running:**
```bash
oc describe pod <pod-name> -n openshift-sriov-network-operator
oc logs <pod-name> -n openshift-sriov-network-operator -f
```

### 2. Check If VF Resources Are Advertised on Nodes
```bash
# Check all nodes for the resource
oc get nodes -o json | jq '.items[] | {
  name: .metadata.name,
  capacity: .status.capacity."openshift.io/cx7anl244",
  allocatable: .status.allocatable."openshift.io/cx7anl244"
}'
```

**Expected Output:**
```json
{
  "name": "worker-0",
  "capacity": "2",
  "allocatable": "2"
}
{
  "name": "worker-1",
  "capacity": "0",
  "allocatable": null
}
```

**If `allocatable` is "0" or missing:** Resources not available!

### 3. Check Node Allocatable in Detail
```bash
oc describe node worker-0 | grep -A 30 "Allocatable"
```

**Look for:** `openshift.io/cx7anl244  2`

**If NOT present:** Resources not advertised on node

---

## 📋 Detailed Diagnosis Steps

### Step 1: Verify SRIOV Network & Policy Exist
```bash
# Check if SRIOV network exists
oc get sriovnetwork -A
oc get sriovnetwork cx7anl244 -n openshift-sriov-network-operator -o yaml

# Check if SRIOV policy exists
oc get sriovnetworknodepolicy -A
oc get sriovnetworknodepolicy cx7anl244 -n openshift-sriov-network-operator -o yaml
```

**Look for:**
- `SriovNetwork` exists in `openshift-sriov-network-operator` namespace
- `SriovNetworkNodePolicy` exists with status "Succeeded"
- Policy has correct `numVfs` (should be 2 or more)

### Step 2: Check NetworkAttachmentDefinition Created
```bash
# Check if NAD exists in test namespace
oc get networkattachmentdefinition -n e2e-25959-cx7anl244
oc get networkattachmentdefinition cx7anl244 -n e2e-25959-cx7anl244 -o yaml
```

**Expected:** NAD should exist in test namespace

### Step 3: Verify Physical Function (PF) on Nodes
```bash
# Check if interface exists on node
oc debug node/worker-0 -- chroot /host lspci | grep -i mellanox
oc debug node/worker-0 -- chroot /host ethtool -i ens2f0np0
```

**Expected:**
```
15:00.0 Ethernet controller: Mellanox Technologies MT42822 BlueField-2 integrated ConnectX-6 DX network controller
```

### Step 4: Check SR-IOV VF Configuration on Node
```bash
# Check how many VFs are configured
oc debug node/worker-0 -- chroot /host \
  cat /sys/class/net/ens2f0np0/device/sriov_numvfs
```

**Expected:** `2` (or number of VFs you configured)

**If shows "0":** VFs not configured on node!

### Step 5: Check Node Resources Status
```bash
# Full node capacity and allocatable
oc describe node worker-0 | grep -A 40 "Allocated resources"
oc get node worker-0 -o json | jq '.status.allocatable'
```

**Look for:** Resources matching your device name

### Step 6: Check SRIOV Operator Logs
```bash
# Get main operator logs
oc logs -n openshift-sriov-network-operator -l app=sriov-network-operator --tail=100

# Get config daemon logs
oc logs -n openshift-sriov-network-operator -l app=sriov-network-config-daemon --tail=100

# Get webhook logs
oc logs -n openshift-sriov-network-operator -l app=sriov-network-webhook --tail=100
```

**Look for:** Errors about resource injection, policy application, or VF configuration

### Step 7: Check MachineConfigPool Status
```bash
# Check if any updates are pending
oc get mcp -o wide
oc describe mcp worker
```

**Expected:** All pools should show "Updated" and not be in progress

**If updating:** Wait for updates to complete before retrying test

### Step 8: Check Device Events
```bash
# Get events in SRIOV namespace
oc get events -n openshift-sriov-network-operator --sort-by='.lastTimestamp'

# Get events in test namespace
oc get events -n e2e-25959-cx7anl244 --sort-by='.lastTimestamp'
```

**Look for:** Warnings or errors related to network/pod creation

---

## 🧪 Test Pod Scheduling Issue

### Check Why Pods Can't Schedule

```bash
# Check pod status
oc describe pod server -n e2e-25959-cx7anl244

# Look for this section:
# Events:
#   Type     Reason            Age    From
#   ----     ------            ----   ----
#   Warning  FailedScheduling  2m27s  default-scheduler
#            0/7 nodes are available: ... 4 Insufficient openshift.io/cx7anl244
```

### Check Pod Requirements vs Node Capacity

```bash
# Check what the pod is requesting
oc get pod server -n e2e-25959-cx7anl244 -o yaml | grep -A 20 "resources:"

# Check what nodes have available
oc get nodes -o json | jq '.items[] | {
  name: .metadata.name,
  allocatable: .status.allocatable."openshift.io/cx7anl244"
}'
```

**Compare:**
- Pod requests: `openshift.io/cx7anl244: 1`
- Node allocatable: Should be >= 1

---

## 🔧 Quick Reference Commands

### View All Resources on All Nodes
```bash
oc get nodes -o json | \
  jq -r '.items[] | "\(.metadata.name): capacity=\(.status.capacity | to_entries[] | select(.key | contains("openshift.io")) | "\(.key):\(.value)") allocatable=\(.status.allocatable | to_entries[] | select(.key | contains("openshift.io")) | "\(.key):\(.value)")"'
```

### Check Resource Availability Per Node
```bash
# Compact view
oc get nodes -o custom-columns=NAME:.metadata.name,\
SRIOV_CAPACITY:.status.capacity.openshift\\.io/cx7anl244,\
SRIOV_ALLOCATABLE:.status.allocatable.openshift\\.io/cx7anl244
```

### Monitor Resource Changes
```bash
# Watch node allocatable in real-time
watch 'oc get nodes -o custom-columns=NAME:.metadata.name,\
SRIOV_CAPACITY:.status.capacity.openshift\\.io/cx7anl244,\
SRIOV_ALLOCATABLE:.status.allocatable.openshift\\.io/cx7anl244'
```

---

## ⚠️ Common Issues & Fixes

### Issue 1: Resources Not Advertised (Allocatable = null)

**Symptoms:**
- `oc get nodes` shows no SRIOV resources
- Pods stuck in Pending with "Insufficient resources"

**Root Causes:**
1. SRIOV operator not running
2. Policy not applied (status != Succeeded)
3. PF interface down or misconfigured
4. Network plugin not injecting resources

**Fix:**
```bash
# 1. Restart operator
oc rollout restart deployment sriov-network-operator \
  -n openshift-sriov-network-operator

# 2. Wait for pods to restart
oc wait --for=condition=Ready pods \
  -l app=sriov-network-operator \
  -n openshift-sriov-network-operator \
  --timeout=5m

# 3. Check resources appear
oc get nodes -o json | jq '.items[].status.allocatable."openshift.io/cx7anl244"'
```

### Issue 2: All Resources Allocated (Allocatable = 0)

**Symptoms:**
- `oc get nodes` shows capacity=2, allocatable=0
- All VFs in use by other pods

**Root Cause:** Resource exhaustion

**Fix:**
```bash
# Check which pods are using resources
oc get pods -A -o json | \
  jq '.items[] | select(.spec.containers[].resources.limits."openshift.io/cx7anl244" != null) | {namespace: .metadata.namespace, name: .metadata.name}'

# Free up resources by deleting unused pods
oc delete pod <pod-name> -n <namespace>

# Or increase VF count in policy
oc edit sriovnetworknodepolicy cx7anl244 \
  -n openshift-sriov-network-operator
# Increase numVfs from 2 to 4 (or higher)
```

### Issue 3: VFs Not Configured on Node

**Symptoms:**
- `/sys/class/net/ens2f0np0/device/sriov_numvfs` shows "0"
- sriov-config-daemon logs show errors

**Root Cause:** Policy not applied due to node reboot or update

**Fix:**
```bash
# Check policy status
oc get sriovnetworknodepolicy cx7anl244 \
  -n openshift-sriov-network-operator -o yaml \
  | grep -A 5 status

# If status != Succeeded, restart config daemon on affected node
oc delete pod -l app=sriov-network-config-daemon \
  -n openshift-sriov-network-operator

# Wait for it to restart
oc wait --for=condition=Ready pods \
  -l app=sriov-network-config-daemon \
  -n openshift-sriov-network-operator \
  --timeout=5m

# Check if VFs are now configured
oc debug node/worker-0 -- chroot /host \
  cat /sys/class/net/ens2f0np0/device/sriov_numvfs
```

---

## 📊 Complete Diagnostic Script

Save this as `diagnose-sriov.sh`:

```bash
#!/bin/bash

echo "=== SRIOV Operator Status ==="
oc get pods -n openshift-sriov-network-operator

echo ""
echo "=== SRIOV Networks ==="
oc get sriovnetwork -A

echo ""
echo "=== SRIOV Policies ==="
oc get sriovnetworknodepolicy -A

echo ""
echo "=== Node Resources ==="
oc get nodes -o custom-columns=NAME:.metadata.name,\
SRIOV_CAP:.status.capacity.openshift\\.io/cx7anl244,\
SRIOV_ALLOC:.status.allocatable.openshift\\.io/cx7anl244

echo ""
echo "=== NetworkAttachmentDefinitions ==="
oc get networkattachmentdefinition -A

echo ""
echo "=== Pending Pods ==="
oc get pods -A --field-selector=status.phase=Pending

echo ""
echo "=== Pod Scheduling Issues ==="
oc get events -A --sort-by='.lastTimestamp' | grep -i insufficient

echo ""
echo "=== Test Namespace ==="
kubectl get ns | grep e2e-
```

**Run it:**
```bash
chmod +x diagnose-sriov.sh
./diagnose-sriov.sh
```

---

## ✅ Verification Checklist

After implementing the fix or making changes, verify:

- [ ] SRIOV operator pods are Running
- [ ] SriovNetworkNodePolicy status is "Succeeded"
- [ ] Node allocatable shows VF resources > 0
- [ ] `/sys/class/net/*/device/sriov_numvfs` shows VF count > 0
- [ ] NetworkAttachmentDefinition exists in test namespace
- [ ] No pending MachineConfig updates
- [ ] Test pod can be created (transitions from Pending)
- [ ] Pod becomes Ready (within 5 minutes)

---

## 🎯 Next Steps

1. **Run the Quick Diagnosis** (steps 1-3 above)
2. **Identify the issue** from the checks
3. **Apply the appropriate fix**
4. **Re-run test** to verify fix works
5. **If still failing**, run Detailed Diagnosis (steps 1-8)

## 📞 When to Escalate

If after running all diagnostic steps:
- SRIOV operator is Running but resources still not advertised
- MachineConfig pool stuck in updating
- Node won't recover after daemon restart

**Then check:**
- SRIOV operator logs for detailed error messages
- Kubernetes API server logs for resource errors
- Network connectivity between nodes and masters
- Disk space on nodes (may prevent daemon from running)

---

**Last Updated:** November 6, 2025  
**For Test Case:** 25959 - SR-IOV VF with spoof checking enabled

