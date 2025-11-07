# Quick Debug Commands for SR-IOV Test Failures

## Problem: NetworkAttachmentDefinition Not Being Deleted (Lines 823-845)

### One-Liner Status Check
```bash
# Check NAD and SriovNetwork status
oc get net-attach-def cx7anl244 -n e2e-25959-cx7anl244 && \
oc get sriovnetwork cx7anl244 -n openshift-sriov-network-operator && \
oc get pods -n openshift-sriov-network-operator
```

---

## Step-by-Step Debugging

### **Step 1: Does the NAD Exist?**
```bash
NAD_NAME="cx7anl244"
TEST_NS="e2e-25959-cx7anl244"

# Simple check
oc get net-attach-def $NAD_NAME -n $TEST_NS

# Detailed view
oc get net-attach-def $NAD_NAME -n $TEST_NS -o yaml
```

### **Step 2: Is SriovNetwork Still There?**
```bash
NET_NAME="cx7anl244"
OP_NS="openshift-sriov-network-operator"

# Check if network exists
oc get sriovnetwork $NET_NAME -n $OP_NS

# Check what's blocking deletion (finalizers)
oc get sriovnetwork $NET_NAME -n $OP_NS -o jsonpath='{.metadata.finalizers}' | jq
```

### **Step 3: Is the SR-IOV Operator Running?**
```bash
OP_NS="openshift-sriov-network-operator"

# Check all operator pods
oc get pods -n $OP_NS -o wide

# Specifically check the main operator
oc get pods -n $OP_NS -l app=sriov-network-operator -o wide

# Check the webhook (handles NAD creation/deletion)
oc get pods -n $OP_NS -l app=operator-webhook -o wide
```

### **Step 4: Check Operator Logs for Errors**
```bash
OP_NS="openshift-sriov-network-operator"

# Last 100 lines of operator logs
oc logs -n $OP_NS -l app=sriov-network-operator --tail=100

# Follow logs in real-time
oc logs -f -n $OP_NS -l app=sriov-network-operator --all-containers=true

# Check webhook logs
oc logs -n $OP_NS -l app=operator-webhook --tail=50
```

### **Step 5: Check for Blocking Finalizers**
```bash
TEST_NS="e2e-25959-cx7anl244"
NAD_NAME="cx7anl244"
OP_NS="openshift-sriov-network-operator"

# Check NAD finalizers
echo "NAD Finalizers:"
oc get net-attach-def $NAD_NAME -n $TEST_NS -o jsonpath='{.metadata.finalizers}'

echo -e "\nSriovNetwork Finalizers:"
oc get sriovnetwork $NAD_NAME -n $OP_NS -o jsonpath='{.metadata.finalizers}'
```

### **Step 6: Check Events**
```bash
TEST_NS="e2e-25959-cx7anl244"
OP_NS="openshift-sriov-network-operator"

# Events in test namespace
echo "=== Test Namespace Events ==="
oc get events -n $TEST_NS --sort-by='.lastTimestamp' | tail -20

# Events in operator namespace
echo -e "\n=== Operator Namespace Events ==="
oc get events -n $OP_NS --sort-by='.lastTimestamp' | tail -20
```

### **Step 7: Check NAD Owner References**
```bash
TEST_NS="e2e-25959-cx7anl244"
NAD_NAME="cx7anl244"

# Should show SriovNetwork as owner (for cascading delete)
oc get net-attach-def $NAD_NAME -n $TEST_NS -o yaml | grep -A 10 "ownerReferences"
```

---

## Manual Cleanup (If Stuck)

### **Force Delete NAD (if stuck)**
```bash
TEST_NS="e2e-25959-cx7anl244"
NAD_NAME="cx7anl244"

# Try normal delete
oc delete net-attach-def $NAD_NAME -n $TEST_NS

# If stuck, remove finalizers
oc patch net-attach-def $NAD_NAME -n $TEST_NS \
  -p '{"metadata":{"finalizers":[]}}' --type=merge

# Then delete
oc delete net-attach-def $NAD_NAME -n $TEST_NS
```

### **Force Delete SriovNetwork (if stuck)**
```bash
OP_NS="openshift-sriov-network-operator"
NET_NAME="cx7anl244"

# Remove finalizers
oc patch sriovnetwork $NET_NAME -n $OP_NS \
  -p '{"metadata":{"finalizers":[]}}' --type=merge

# Then delete
oc delete sriovnetwork $NET_NAME -n $OP_NS
```

---

## Complete Diagnostic Bundle (Copy-Paste Ready)

```bash
#!/bin/bash
set -e

NAD_NAME="${1:-cx7anl244}"
TEST_NS="${2:-e2e-25959-cx7anl244}"
OP_NS="openshift-sriov-network-operator"

echo "========================================="
echo "SR-IOV Network Removal Diagnostics"
echo "========================================="
echo "NAD: $NAD_NAME"
echo "Test Namespace: $TEST_NS"
echo "Operator Namespace: $OP_NS"
echo ""

echo "[1/8] NAD Status"
echo "---"
oc get net-attach-def $NAD_NAME -n $TEST_NS -o wide 2>/dev/null || echo "NAD NOT FOUND"
echo ""

echo "[2/8] NAD Finalizers"
echo "---"
oc get net-attach-def $NAD_NAME -n $TEST_NS -o jsonpath='{.metadata.finalizers}' 2>/dev/null || echo "N/A"
echo ""

echo "[3/8] SriovNetwork Status"
echo "---"
oc get sriovnetwork $NAD_NAME -n $OP_NS -o wide 2>/dev/null || echo "SRIOV NETWORK NOT FOUND"
echo ""

echo "[4/8] SriovNetwork Finalizers"
echo "---"
oc get sriovnetwork $NAD_NAME -n $OP_NS -o jsonpath='{.metadata.finalizers}' 2>/dev/null || echo "N/A"
echo ""

echo "[5/8] SR-IOV Operator Pods"
echo "---"
oc get pods -n $OP_NS --no-headers | grep -E "(sriov-network-operator|operator-webhook)" || echo "Pods not running"
echo ""

echo "[6/8] Operator Pod Status"
echo "---"
oc get pods -n $OP_NS -o wide | head -10
echo ""

echo "[7/8] Recent Operator Logs"
echo "---"
oc logs -n $OP_NS -l app=sriov-network-operator --tail=30 2>/dev/null | tail -10
echo ""

echo "[8/8] Recent Events"
echo "---"
echo "Test Namespace Events:"
oc get events -n $TEST_NS --sort-by='.lastTimestamp' 2>/dev/null | tail -5 || echo "No events"
echo ""
echo "Operator Namespace Events:"
oc get events -n $OP_NS --sort-by='.lastTimestamp' 2>/dev/null | tail -5 || echo "No events"
```

Save as `sriov-debug.sh`, then run:
```bash
chmod +x sriov-debug.sh
./sriov-debug.sh cx7anl244 e2e-25959-cx7anl244
```

---

## Common Issues & Quick Fixes

| Symptom | Command | Fix |
|---------|---------|-----|
| `NAD NOT FOUND` but test waits | Already deleted ✓ | Increase timeout in code |
| NAD exists but has finalizers | `oc get net-attach-def ... -o jsonpath='{.metadata.finalizers}'` | Remove finalizers: `oc patch ... -p '{"metadata":{"finalizers":[]}}'` |
| SriovNetwork stuck "Deleting" | `oc get sriovnetwork ... -o yaml \| grep deletionTimestamp` | Remove finalizers from SriovNetwork |
| Operator not running | `oc get pods -n openshift-sriov-network-operator` | Restart: `oc rollout restart deployment/sriov-network-operator -n openshift-sriov-network-operator` |
| Webhook not responding | `oc logs -l app=operator-webhook` | Check webhook pod logs for errors |
| Permission denied | `oc get net-attach-def ... --v=10` | Check RBAC: `oc get clusterrole,clusterrolebinding -l sriov` |

---

## What to Check First (Priority Order)

1. **NAD exists?** → Check if operator created it
2. **Operator running?** → Check pod status
3. **Operator logs?** → Check for errors
4. **Finalizers?** → Check what's blocking deletion
5. **Events?** → Check what happened
6. **Owner references?** → Check if NAD is owned by SriovNetwork

---

## Test Case Specific

### For Test Case 25959 (Spoof Checking On)
```bash
# Exact commands for this test
TEST_ID="25959"
DEVICE="cx7anl244"

NAD_NS="e2e-${TEST_ID}-${DEVICE}"
echo "Checking: $NAD_NS"
oc get net-attach-def -n $NAD_NS
oc get events -n $NAD_NS --sort-by='.lastTimestamp'
```

---

## Additional Resources

- Operator namespace: `openshift-sriov-network-operator`
- Test namespace pattern: `e2e-<test-id>-<device-name>`
- NAD name matches: SriovNetwork name and resource name
- Key resource: `SriovNetwork` → creates → `NetworkAttachmentDefinition`


