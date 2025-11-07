# SR-IOV Network Removal Failure Analysis
## Lines 823-845: NetworkAttachmentDefinition Deletion Timeout

---

## Problem Summary

The test is failing during **cleanup** when trying to wait for the `NetworkAttachmentDefinition` (NAD) to be deleted after removing the `SriovNetwork` resource.

```
[FAILED] Timed out after 180.002s.
Failed to wait for NetworkAttachmentDefinition cx7anl244 in namespace e2e-25959-cx7anl244. 
Ensure the SRIOV policy exists and is properly configured.

Error: networkattachmentdefinition object cx7anl244 does not exist in namespace e2e-25959-cx7anl244
```

---

## Execution Flow (What's Happening)

### 1. **Test Execution Completes Successfully**
   - Test runs and verifies SR-IOV configuration
   - The main test logic passes

### 2. **Cleanup Phase Starts** (where the failure occurs)
   - Line 823: `STEP: Removing SRIOV network cx7anl244`
   - Command: `oc delete sriovnetwork cx7anl244 -n openshift-sriov-network-operator`
   - Line 825: `STEP: Waiting for SRIOV network cx7anl244 to be deleted` ✓ This succeeds
   - Line 826: `STEP: Waiting for NetworkAttachmentDefinition cx7anl244 to be deleted in namespace e2e-25959-cx7anl244`
   - **Line 827: [FAILED]** - Times out after 180 seconds (3 minutes)

---

## Root Cause Analysis

### Code Location: `/root/eco-gotests/tests/sriov/helpers.go:583-609`

```go
// Wait for NAD to be deleted in the target namespace
if targetNamespace != sriovOpNs {
    By(fmt.Sprintf("Waiting for NetworkAttachmentDefinition %s to be deleted in namespace %s", 
       name, targetNamespace))
    err = wait.PollUntilContextTimeout(
        context.TODO(),
        2*time.Second,           // Poll interval
        1*time.Minute,           // TIMEOUT = 60 SECONDS ⚠️
        true,
        func(ctx context.Context) (bool, error) {
            _, pullErr := nad.Pull(getAPIClient(), name, targetNamespace)
            if pullErr != nil {
                // NAD is deleted (we got an error/not found), which is what we want
                return true, nil
            }
            // NAD still exists, keep waiting
            GinkgoLogr.Info("NetworkAttachmentDefinition still exists", ...)
            return false, nil
        })
    
    if err != nil {
        // ERROR HAPPENS HERE at line 607-608
        Expect(err).ToNot(HaveOccurred(),
            "NetworkAttachmentDefinition %s was not deleted from namespace %s within timeout", 
            name, targetNamespace)
    }
}
```

### The Problem: **Mismatch in Timeout Values**

Looking at the error message:
- **Actual error timeout**: **180.002 seconds** (3 minutes + margin)
- **Code timeout**: **1 minute** (60 seconds at line 589)

This means the polling is continuing **past the 1-minute timeout** and the outer `Eventually` is timing out with a **180-second (3-minute) limit**.

**The NAD is NOT being deleted by the SR-IOV operator after the SriovNetwork CR is deleted.**

---

## Why the NAD Isn't Being Deleted

When a `SriovNetwork` resource is deleted, the SR-IOV operator should delete the corresponding `NetworkAttachmentDefinition`. But this isn't happening because:

### Potential Causes:

1. **SR-IOV Operator Webhook/Controller Issue**
   - The operator's webhook that watches for SriovNetwork deletion isn't responding
   - The controller that creates/deletes NADs is stuck or crashed

2. **NAD Ownership Chain Broken**
   - The NAD doesn't have proper owner references to the SriovNetwork
   - Kubernetes doesn't cascade delete the NAD when SriovNetwork is deleted

3. **Finalists/Finalizers on Resources**
   - SriovNetwork might have finalizers preventing cleanup
   - NAD might be protected by a finalizer

4. **Operator Pod Issues**
   - SR-IOV operator pods are not running
   - Operator pod is in `CrashLoopBackOff`
   - Network connectivity issues between operator and API server

5. **Resource Namespace Issues**
   - Target namespace (`e2e-25959-cx7anl244`) exists
   - NAD exists but operator doesn't have permissions to delete it
   - NAD is in a different namespace than expected

---

## Debugging Commands

### 1. **Check if NAD Actually Exists**
```bash
# Check if the NAD was created in the first place
oc get net-attach-def cx7anl244 -n e2e-25959-cx7anl244 -o yaml

# List all NADs in the namespace
oc get net-attach-def -n e2e-25959-cx7anl244
```

### 2. **Check SriovNetwork Deletion Status**
```bash
# Check if SriovNetwork still exists
oc get sriovnetwork cx7anl244 -n openshift-sriov-network-operator -o yaml

# Look for finalizers (these can block deletion)
oc get sriovnetwork cx7anl244 -n openshift-sriov-network-operator \
  -o jsonpath='{.metadata.finalizers}'

# Check deletion timestamp (indicating it's in terminating state)
oc get sriovnetwork cx7anl244 -n openshift-sriov-network-operator \
  -o jsonpath='{.metadata.deletionTimestamp}'
```

### 3. **Check SR-IOV Operator Status**
```bash
# Check operator pods are running
oc get pods -n openshift-sriov-network-operator -o wide

# Check operator logs for errors
oc logs -n openshift-sriov-network-operator \
  -l app=sriov-network-operator --tail=100

# Check if webhook is running
oc get pods -n openshift-sriov-network-operator \
  -l app=operator-webhook -o wide
```

### 4. **Check NAD Owner References**
```bash
# Check if NAD has owner reference to SriovNetwork
oc get net-attach-def cx7anl244 -n e2e-25959-cx7anl244 -o yaml | grep -A 10 ownerReferences

# Compare with a working NAD (from another test)
oc get net-attach-def -A -o yaml | head -50
```

### 5. **Check for Finalizers on NAD**
```bash
# Check if NAD has finalizers (can block deletion)
oc get net-attach-def cx7anl244 -n e2e-25959-cx7anl244 \
  -o jsonpath='{.metadata.finalizers}'
```

### 6. **Check Events in Target Namespace**
```bash
# Get events in the test namespace
oc get events -n e2e-25959-cx7anl244 --sort-by='.lastTimestamp' | tail -30

# Get events in operator namespace
oc get events -n openshift-sriov-network-operator --sort-by='.lastTimestamp' | tail -30
```

### 7. **Manual Test of NAD Deletion**
```bash
# Try to manually delete the NAD
oc delete net-attach-def cx7anl244 -n e2e-25959-cx7anl244 -v 8

# Check if it deletes immediately or hangs
oc get net-attach-def cx7anl244 -n e2e-25959-cx7anl244 -o yaml | grep deletionTimestamp
```

---

## Quick Diagnosis Script

```bash
#!/bin/bash
set -e

NAMESPACE="e2e-25959-cx7anl244"
NAD_NAME="cx7anl244"
OP_NS="openshift-sriov-network-operator"

echo "=== CHECK 1: NAD EXISTS? ==="
oc get net-attach-def $NAD_NAME -n $NAMESPACE -o yaml 2>&1 || echo "NAD does not exist (good)"

echo -e "\n=== CHECK 2: SRIOV NETWORK EXISTS? ==="
oc get sriovnetwork $NAD_NAME -n $OP_NS -o yaml 2>&1 | head -20 || echo "Network not found"

echo -e "\n=== CHECK 3: SRIOV NETWORK FINALIZERS ==="
oc get sriovnetwork $NAD_NAME -n $OP_NS -o jsonpath='{.metadata.finalizers}' 2>&1

echo -e "\n=== CHECK 4: NAD FINALIZERS ==="
oc get net-attach-def $NAD_NAME -n $NAMESPACE -o jsonpath='{.metadata.finalizers}' 2>&1 || echo "NAD not found"

echo -e "\n=== CHECK 5: OPERATOR PODS STATUS ==="
oc get pods -n $OP_NS -l app=sriov-network-operator -o wide

echo -e "\n=== CHECK 6: OPERATOR LOGS (last 50 lines) ==="
oc logs -n $OP_NS -l app=sriov-network-operator --tail=50

echo -e "\n=== CHECK 7: RECENT EVENTS IN OPERATOR NS ==="
oc get events -n $OP_NS --sort-by='.lastTimestamp' | tail -10

echo -e "\n=== CHECK 8: NAD OWNER REFERENCES ==="
oc get net-attach-def $NAD_NAME -n $NAMESPACE -o yaml | grep -A 5 ownerReferences || echo "No owner references found"
```

---

## Fix Strategies

### **Immediate Fix: Extend Timeout**
Increase the timeout in `helpers.go:589` from `1*time.Minute` to `3*time.Minute` or `5*time.Minute`:

```go
err = wait.PollUntilContextTimeout(
    context.TODO(),
    2*time.Second,
    5*time.Minute,  // Increased from 1*time.Minute
    true,
    func(ctx context.Context) (bool, error) {
        // ... existing code ...
    })
```

### **Better Fix: Manual Cleanup**
If the operator won't delete the NAD, the test should manually delete it:

```go
// If NAD still exists after SriovNetwork deletion, manually delete it
By(fmt.Sprintf("Manually cleaning up NetworkAttachmentDefinition %s if it still exists", name))
nadBuilder, err := nad.Pull(getAPIClient(), name, targetNamespace)
if err == nil && nadBuilder.Exists() {
    GinkgoLogr.Info("Manually deleting NAD", "name", name, "namespace", targetNamespace)
    err = nadBuilder.Delete()
    if err != nil {
        GinkgoLogr.Info("Failed to manually delete NAD", "error", err)
    }
}
```

### **Root Fix: Fix SR-IOV Operator**
- Check SR-IOV operator version and logs
- Verify webhook is properly configured
- Check operator RBAC permissions
- Restart operator pods if necessary

---

## Timeline Summary

| Time | Event |
|------|-------|
| T+0s | Test execution completes |
| T+0s | Cleanup phase starts - delete SriovNetwork |
| T+30s | SriovNetwork CR deletion succeeds ✓ |
| T+32s | Start waiting for NAD deletion (timeout: 60s) |
| T+92s | Poll timeout expires (after 60s) ❌ |
| T+180s | Outer Eventually timeout (3 min) - **TEST FAILS** |

---

## Prevention

1. **Monitor operator logs** during test execution
2. **Add pre-flight check** to verify operator is healthy before running tests
3. **Implement retry logic** for cleanup operations
4. **Add manual cleanup fallback** if operator cleanup fails
5. **Increase timeout** to account for slow cluster response times

