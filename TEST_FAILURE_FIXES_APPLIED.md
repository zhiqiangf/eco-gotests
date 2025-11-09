# SR-IOV Test Failures - Root Causes and Fixes Applied

**Date:** November 9, 2025  
**Status:** ✅ Fixed - Operator Restored and Tests Can Resume

---

## Executive Summary

During the 20-minute test checkpoint, tests began failing due to the SR-IOV operator being removed from the cluster (by one of the test cases) and not being properly restored. We identified and fixed the root causes.

---

## Root Cause Analysis

### Primary Issue: Missing SR-IOV Operator

**Symptom:** Multiple tests failing with:
- "CSV should be in Succeeded phase" 
- "pod client-* does not have network status annotation"
- "networkattachmentdefinition object * does not exist"

**Root Cause:**
One of the earlier test cases (likely `test_sriov_components_cleanup_on_removal` or `test_sriov_operator_reinstallation_functionality`) removed the SR-IOV operator via OLM but failed to properly restore it. This left the cluster in a broken state where:
- CSV (ClusterServiceVersion) was missing
- Main operator pod was not running
- NetworkAttachmentDefinitions were not being created
- SR-IOV network creation was failing

### Secondary Issues Identified

1. **CSV Missing**
   - ClusterServiceVersion was deleted and not recreated
   - This prevented the operator from being recognized by OLM

2. **Operator Pod Missing**
   - `sriov-network-operator-*` deployment was absent
   - The operator couldn't reconcile policies or create networks

3. **NAD Creation Blocked**
   - NetworkAttachmentDefinition resources depend on the operator running
   - Without the operator, SR-IOV networks cannot create NADs
   - Test pods cannot attach to networks without NADs

---

## Fixes Applied

### Fix #1: Operator Subscription Deletion and Recreation

**Command:**
```bash
oc delete subscription sriov-network-operator -n openshift-sriov-network-operator

# Recreated subscription
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: sriov-network-operator
  namespace: openshift-sriov-network-operator
spec:
  channel: stable
  name: sriov-network-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
```

**Result:**
- ✅ CSV automatically created and reached "Succeeded" phase
- ✅ Operator pod (`sriov-network-operator-7d5466cf46-tcrbl`) started running
- ✅ All supporting pods verified running

### Fix #2: Verified SriovOperatorConfig Still Present

**Status:** ✅ Configuration was properly retained from earlier session

```
NAME      AGE
default   55m
```

The `SriovOperatorConfig` default object remained intact, allowing the operator to start with correct configuration.

### Fix #3: Verified SR-IOV Policies Intact

**Policies found:**
```
NAME          AGE
cx6dxanl244   110m
cx7anl244     4h32m
```

SR-IOV policies were retained and immediately began working once the operator restarted.

### Fix #4: Verified Network Creation Resumed

**Result:** NetworkAttachmentDefinitions automatically created:
```
NAMESPACE                  NAME              AGE
e2e-70820-cx7anl244        70820-cx7anl244   50s
```

Once the operator restarted, it immediately began creating NADs for the existing SR-IOV networks.

---

## Test Failure Prevention

### Root Issue in Tests: Incomplete Operator Restoration

The tests that remove and reinstall the operator have improved restoration logic (added in recent commits), but one test's restoration may have been incomplete. The issue was:

1. Test removes operator via CSV deletion
2. Test attempts to restore operator
3. **Restoration succeeds at surface level but subscription wasn't fully recreated**
4. When OLM removed the old subscription, the new one wasn't properly established
5. Subsequent tests found operator missing

### Code Analysis

Files with operator removal/restoration logic:
- `tests/sriov/sriov_lifecycle_test.go` - Has restoration logic
- `tests/sriov/sriov_reinstall_test.go` - Has restoration logic  
- `tests/sriov/sriov_advanced_scenarios_test.go` - May need restoration logic review

**Required Fix:**
The test restoration logic needs to ensure the subscription itself is recreated, not just waiting for CSV/pods. The current code attempts manual restoration but may have timing issues.

---

## Verification After Fixes

### Operator Status: ✅ HEALTHY

```
NAME                                          DISPLAY                   VERSION               PHASE
sriov-network-operator.v4.20.0-202510221121   SR-IOV Network Operator   4.20.0-202510221121   Succeeded
```

### All Supporting Pods: ✅ RUNNING

```
network-resources-injector-*           (3/3)
operator-webhook-*                     (3/3)
sriov-device-plugin-*                  (1/1)
sriov-network-config-daemon-*          (4/4)
sriov-network-operator-*               (1/1)
```

### Node States: ✅ SYNCED

```
NAME                                           SYNC STATUS
worker-0                                       Succeeded
worker-1                                       Succeeded
worker-2                                       Succeeded
wsfd-advnetlab244.sriov.openshift-qe.sdn.com   Succeeded
```

### Network Creation: ✅ WORKING

SR-IOV networks are being created and NADs are being generated automatically.

---

## Test Resumption

Tests can now resume and should progress without the operator-related failures. However, there may be other test-specific issues such as:

1. **Pod network status annotations** - May need NADs to fully settle
2. **MachineConfigPool updates** - May need more time to complete
3. **Network connectivity** - Pods need time to get SR-IOV interfaces

---

## Recommended Code Changes

### In `tests/sriov/sriov_lifecycle_test.go` and `tests/sriov/sriov_reinstall_test.go`

The operator restoration code should be enhanced to:

1. **Verify subscription is properly created:**
   ```go
   // After restoring subscription, verify it exists
   Eventually(func() error {
       return verifySubscriptionExists(apiClient, "sriov-network-operator")
   }).Should(BeNil())
   ```

2. **Ensure CSV reaches Succeeded phase:**
   ```go
   // Wait for CSV to be in Succeeded phase
   Eventually(func() error {
       return verifyCSVSucceeded(apiClient, "openshift-sriov-network-operator")
   }).Should(BeNil())
   ```

3. **Verify operator pod is running:**
   ```go
   // Verify main operator pod is running
   Eventually(func() error {
       return verifyOperatorPodRunning(apiClient, "openshift-sriov-network-operator")
   }).Should(BeNil())
   ```

---

## Conclusion

The test failures were caused by incomplete operator restoration after removal. The cluster's SR-IOV infrastructure was intact (policies, configurations) but the operator itself was missing. By properly recreating the subscription, the operator automatically reconciled back to a healthy state.

**Key Lesson:** When removing and restoring operators in tests, must verify:
1. ✅ Subscription is recreated
2. ✅ CSV reaches Succeeded phase
3. ✅ Operator pods are running
4. ✅ Configuration objects are present

All of these are now verified and in place.

