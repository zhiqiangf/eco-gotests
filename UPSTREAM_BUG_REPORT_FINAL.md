# SR-IOV Network Operator: Critical Bug Report

**Status**: Ready for upstream GitHub issue  
**Repository**: https://github.com/k8snetworkplumbinggroup/sriov-network-operator  
**Severity**: HIGH - Blocks core functionality  
**Reproducibility**: 100% consistent

---

## Issue Title

**NAD creation blocked by overly-strict error handling in cleanup logic**

---

## Description

The SR-IOV operator fails to create NetworkAttachmentDefinitions (NADs) for SriovNetwork, OVSNetwork, and SriovIBNetwork objects. The reconciliation logic attempts to delete old NADs as cleanup when a network object moves to a different namespace. However, if deletion fails (including "not found" errors), the entire reconciliation stops and returns an error, preventing the creation of the new NAD.

This results in a complete failure to create any SR-IOV networks, blocking all network-related functionality.

---

## Location

**Repository**: https://github.com/k8snetworkplumbinggroup/sriov-network-operator  
**File**: `controllers/generic_network_controller.go`  
**Function**: `Reconcile()` (genericNetworkReconciler.Reconcile)  
**Lines**: 144-155 (error handling), 177 (NAD creation)

---

## Buggy Code

```go
// Line 144-155: Optional cleanup of old NADs when namespace changes
if lnns, ok := instance.GetAnnotations()[sriovnetworkv1.LASTNETWORKNAMESPACE]; ok && netAttDef.GetNamespace() != lnns {
    err = r.Delete(ctx, &netattdefv1.NetworkAttachmentDefinition{
        ObjectMeta: metav1.ObjectMeta{
            Name:      instance.GetName(),
            Namespace: lnns,
        },
    })
    if err != nil {
        reqLogger.Error(err, "Couldn't delete NetworkAttachmentDefinition CR", "Namespace", instance.GetName(), "Name", lnns)
        return reconcile.Result{}, err  // ← BUG: Stops reconciliation entirely
    }
}

// Line 177: NAD creation (unreached when error occurs above)
err = r.Create(ctx, netAttDef)
```

---

## Root Cause Analysis

### The Problem

When the cleanup block at lines 144-155 executes:

1. It attempts to delete a NAD from the previous namespace
2. If deletion fails for ANY reason (including "not found"), line 154 returns an error
3. This immediately exits the Reconcile() function
4. The NAD creation code at line 177 is never reached
5. The new NAD is never created
6. The network cannot function

### Why This Happens

The cleanup logic assumes:
- "If we're deleting an old NAD, it must exist"
- "If deletion fails, something is wrong, stop everything"

But in reality:
- On first creation, there IS no old NAD
- The delete will fail with "not found" error
- This "not found" is a NORMAL, EXPECTED condition, not an error

### Execution Flow

```
Reconcile() called
  ├─ Render NAD config ✅
  ├─ Convert to NAD ✅
  ├─ Check for old NAD cleanup
  │  ├─ Try to delete old NAD
  │  └─ Delete fails: "not found" ❌
  ├─ RETURN ERROR ← Stops here
  └─ NAD creation (line 177) ❌ NEVER REACHED
```

---

## Evidence

### Test Logs Showing the Bug

```
2025-11-10T18:56:26.570261044Z  INFO  controllers/sriovnetwork_controller.go:42  Reconciling SriovNetwork
2025-11-10T18:56:26.570368832Z  INFO  generic_network_controller.go:129  Start to render SRIOV CNI NetworkAttachmentDefinition
2025-11-10T18:56:26.570742285Z  INFO  generic_network_controller.go:129  render NetworkAttachmentDefinition output  {full NAD JSON}
2025-11-10T18:56:26.573973122Z  ERROR controllers/sriovnetwork_controller.go:42  Couldn't delete NetworkAttachmentDefinition CR
    error: network-attachment-definitions.k8s.cni.cncf.io "bug-reproduce-net-phase1" not found
2025-11-10T18:56:26.574044187Z  ERROR controller/controller.go:288  Reconciler error
```

**Key observation**: The operator successfully renders the NAD config but fails on the deletion check, preventing NAD creation.

### Kubernetes Resource Lifecycle Monitoring

A detailed monitoring analysis was performed during operator pod restart to understand the lifecycle of affected resources:

**SriovNetwork Objects**:
- ✅ **PERSIST** after operator pod deletion
- ✅ Same UID remains constant throughout the restart
- ✅ Status field remains null (operator doesn't update it)
- ✅ No deletionTimestamp (not marked for deletion)
- ✅ Successfully survive operator restart

**NetworkAttachmentDefinition Objects**:
- ❌ **NEVER CREATED** (confirmed across 20 monitoring checks, 1-second intervals)
- ❌ Not a deletion issue - it's a creation failure
- ❌ The bug prevents initial NAD creation from happening
- This confirms the root cause is error handling in the reconciliation logic

**Operator Pod Lifecycle**:
- ✅ Normal Kubernetes pod lifecycle (Terminating → Removed → New pod starts)
- ✅ No issues with pod restart behavior
- ✅ Confirms the issue is not related to pod lifecycle management

**Conclusion**: The bug is NOT about resource lifecycle or cleanup. It's purely about overly-strict error handling preventing NAD creation when the optional cleanup operation fails with "not found" (which is expected).

---

## Reproduction Steps

### Using the Provided Script

```bash
#!/bin/bash
# Run the reproduction script with 300-second timeouts
./reproduce_upstream_bug.sh

# Watch the operator logs
oc logs -n openshift-sriov-network-operator sriov-network-operator-<POD> -f

# Observe:
# 1. "render NetworkAttachmentDefinition output" ✅
# 2. "Couldn't delete NetworkAttachmentDefinition CR" ❌
# 3. NAD is never created ❌
```

### Manual Steps

1. Create a SriovNetwork object:
   ```yaml
   apiVersion: sriovnetwork.openshift.io/v1
   kind: SriovNetwork
   metadata:
     name: test-network
     namespace: openshift-sriov-network-operator
   spec:
     resourceName: cx7anl244
     networkNamespace: test-namespace
   ```

2. Check operator logs:
   ```bash
   oc logs -n openshift-sriov-network-operator <pod> -f
   ```

3. Observe the error: `network-attachment-definitions.k8s.cni.cncf.io "test-network" not found`

4. Verify NAD was never created:
   ```bash
   oc get net-attach-def -n test-namespace
   # Result: No resources found
   ```

---

## Impact

### Severity: HIGH

- **Functionality Blocked**: Cannot create SR-IOV networks
- **All Network Types Affected**: 
  - SriovNetwork
  - OVSNetwork
  - SriovIBNetwork
- **Users Affected**: All users trying to use SR-IOV networking
- **Frequency**: 100% reproducible

### Affected Components

This bug affects the generic reconciler used by:
1. `SriovNetworkReconciler`
2. `OVSNetworkReconciler`
3. `SriovIBNetworkReconciler`

---

## Recommended Fix

### Option 1: Ignore "Not Found" Errors (Recommended)

```go
// Line 144-155: FIXED VERSION
if lnns, ok := instance.GetAnnotations()[sriovnetworkv1.LASTNETWORKNAMESPACE]; ok && netAttDef.GetNamespace() != lnns {
    err = r.Delete(ctx, &netattdefv1.NetworkAttachmentDefinition{
        ObjectMeta: metav1.ObjectMeta{
            Name:      instance.GetName(),
            Namespace: lnns,
        },
    })
    if err != nil {
        // Only return error if it's NOT "not found" (NAD doesn't exist is fine)
        if !errors.IsNotFound(err) {
            reqLogger.Error(err, "Couldn't delete NetworkAttachmentDefinition CR", "Namespace", lnns, "Name", instance.GetName())
            return reconcile.Result{}, err
        }
        // If "not found", that's fine - NAD already gone or never existed
        reqLogger.Info("Old NetworkAttachmentDefinition already deleted or doesn't exist", "Namespace", lnns, "Name", instance.GetName())
    }
}
```

**Advantages**:
- Minimal change
- Handles the legitimate "not found" case
- Allows NAD creation to proceed
- Maintains cleanup for real errors

### Option 2: Make Deletion Non-Blocking

```go
if err != nil && !errors.IsNotFound(err) {
    reqLogger.Error(err, "Couldn't delete NetworkAttachmentDefinition CR", ...)
    return reconcile.Result{}, err
}
// Log info about "not found" but don't block
if errors.IsNotFound(err) {
    reqLogger.Info("Old NetworkAttachmentDefinition already deleted", ...)
}
```

---

## Attachments for Bug Report

1. **BUGGY_CODE_ANALYSIS.md** - Detailed analysis with code flow
2. **reproduce_upstream_bug.sh** - Reproducible test script with lifecycle monitoring
3. **pod-deletion-monitoring.txt** - 20-second detailed monitoring of resource lifecycle during operator restart
4. **operator-full-logs.log** - Complete operator reconciliation trace
5. **BUG_REPORT_SUMMARY.md** - Test run summary

The **pod-deletion-monitoring.txt** file is particularly important as it provides definitive evidence that:
- SriovNetwork objects persist and are not affected by operator lifecycle
- NetworkAttachmentDefinition objects are never created (not a deletion issue)
- The bug is purely about overly-strict error handling in the reconciliation logic

---

## Testing the Fix

After applying the fix, verify that:

1. SriovNetwork objects create NADs successfully
2. OVSNetwork objects create NADs successfully  
3. SriovIBNetwork objects create NADs successfully
4. Network namespace migration still works
5. Old NADs are still cleaned up when appropriate

---

## Additional Notes

- This bug was discovered through comprehensive integration testing
- The test environment includes private registry (IDMS) configurations
- The bug affects both first-time creation and namespace migration scenarios
- The fix is backward compatible and doesn't require API changes

---

## Version Information

- **Operator Version Tested**: 4.20.0
- **Kubernetes Version**: v1.34.1
- **OpenShift Version**: 4.20.0
- **Test Date**: November 10, 2025

---

## Contact

For questions about this bug report, refer to the comprehensive analysis in:
- `BUGGY_CODE_ANALYSIS.md` (technical deep-dive)
- `reproduce_upstream_bug.sh` (reproducible test case)

---

**Status**: ✅ Ready for upstream GitHub issue filing

