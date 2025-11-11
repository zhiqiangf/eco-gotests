# SR-IOV Test Isolation Analysis

**Date:** November 9, 2025  
**Issue:** Tests that remove the SR-IOV operator are not properly restoring it, causing subsequent tests to fail

---

## Problem Statement

During test execution, certain tests marked as "Disruptive" intentionally remove the SR-IOV operator to test operator lifecycle management. However, these tests are failing to properly restore the operator environment, leaving the cluster in a broken state where subsequent tests cannot execute.

### Affected Tests

1. **`test_sriov_components_cleanup_on_removal`** in `sriov_lifecycle_test.go`
   - Removes the SR-IOV operator namespace and all resources
   - Attempts to restore operator in Phase 4
   - **Issue:** Restoration was failing silently or incompletely

2. **`test_sriov_operator_reinstallation_functionality`** in `sriov_reinstall_test.go`
   - Deletes the operator pod to trigger reinstallation
   - Attempts to restore operator in Phase 2
   - **Issue:** Restoration was waiting for pods but subscription was missing

---

## Root Cause Analysis

### Why Operator Restoration Failed

The original restoration logic had several issues:

1. **Incomplete Subscription Recreation**
   - Only waited for existing subscription to trigger operator deployment
   - Did NOT recreate subscription if it was deleted
   - Without subscription, OLM has no way to know to deploy the operator

2. **Missing SriovOperatorConfig**
   - After restoring subscription, operator still won't function without this critical CRD
   - Without SriovOperatorConfig, operator pods are created but cannot reconcile policies

3. **Insufficient Retry Logic**
   - Original code only waited 30 iterations × 3 seconds = 90 seconds
   - OLM and operator deployment can take longer
   - No progressive logging of attempts

4. **No Fallback for OLM Delays**
   - OLM can be slow to deploy operators from catalog
   - Tests didn't account for catalog source health checks
   - No verification that deployment actually started

### Test Isolation Chain Reaction

```
Test N: Removes operator
    ↓
Restoration fails (operator doesn't come back)
    ↓
Test N+1: Tries to initialize SR-IOV
    ↓
No operator pods → Test hangs waiting for operator
    ↓
Subsequent tests all fail (operator never recovers)
    ↓
Entire test suite blocked
```

---

## Solution Implemented

### Improvements in Version 2 (Just Fixed)

The restored `manuallyRestoreOperator` function now:

1. **Actively Recreates Missing Subscription**
   ```go
   sub, err := getOperatorSubscription(apiClient, sriovOpNs)
   if err != nil {
       // Recreate subscription using oc apply
       // OLM watches subscription and deploys operator
   }
   ```

2. **Ensures SriovOperatorConfig Exists**
   ```go
   // Create if missing - operator won't function without this
   oc apply -f - <<'EOF'
   apiVersion: sriovnetwork.openshift.io/v1
   kind: SriovOperatorConfig
   ...
   ```

3. **Extended Retry with Progress Logging**
   ```go
   for i := 0; i < 40; i++ {  // 40 iterations × 3 seconds = 120 seconds
       // Check for operator pods
       if pods found {
           return success
       }
       // Log progress every 5 iterations
   }
   ```

4. **Stabilization Wait**
   ```go
   // After pods appear, wait 5 more seconds for stabilization
   time.Sleep(5 * time.Second)
   ```

---

## Remaining Test Isolation Concerns

### Current Design Issues

1. **Sequential Test Execution**
   - All tests run in series (Serial flag)
   - One broken test blocks everything after it
   - No isolation between tests

2. **Shared Cluster State**
   - Tests modify node configurations
   - Tests create/delete SR-IOV resources
   - No namespace or resource isolation beyond network namespaces

3. **Operator Deletion Tests**
   - `test_sriov_components_cleanup_on_removal` completely removes operator
   - If this test fails during restoration, all subsequent tests fail
   - Need stronger guarantee of restoration

### Recommendations for Stronger Isolation

#### Short Term (In Code)

1. **Add Pre-Test Verification**
   ```go
   func ensureOperatorReady() {
       if operator not running {
           Fail("Operator not ready - previous test may have failed restoration")
       }
   }
   ```

2. **BeforeEach with Operator Check**
   ```go
   BeforeEach(func() {
       By("Verifying operator is ready for test")
       Expect(chkSriovOperatorStatus(sriovOpNs)).To(Succeed())
   })
   ```

3. **Add Hard Timeout for Operator Restoration**
   ```go
   // If restoration takes too long, fail fast instead of hanging
   timeout := 3 * time.Minute
   ```

#### Medium Term (Test Suite Structure)

1. **Separate Tests by Disruptiveness**
   - Run non-disruptive tests first
   - Run disruptive tests last or in isolation
   - Have cleanup jobs between disruptive tests

2. **Test Groups**
   - Group 1: Basic functionality (non-disruptive)
   - Group 2: Advanced features (non-disruptive)
   - Group 3: Lifecycle tests (disruptive)
   - Group 4: Reinstall tests (disruptive)

3. **Health Check Between Groups**
   ```go
   AfterSuite(func() {
       // Verify operator is healthy
       // Verify all resources are clean
       // Log cluster state
   })
   ```

#### Long Term (CI/CD Improvements)

1. **Parallel Test Execution**
   - Run test suites on separate clusters
   - Eliminate sequential test coupling
   - Faster feedback

2. **Test Environment Snapshots**
   - Save "golden" cluster state
   - Restore between test groups
   - Guarantee clean slate

3. **Operator Health Monitoring**
   - Continuous verification during tests
   - Auto-repair mechanism
   - Alert on degradation

---

## Testing the Fix

### How to Verify Operator Restoration Now Works

1. **Run a Single Disruptive Test**
   ```bash
   go test ./tests/sriov/... -run TestSriovLifecycle -v
   ```

2. **Check Operator After Test**
   ```bash
   oc get pods -n openshift-sriov-network-operator
   ```
   Expected: All operator pods running

3. **Run Multiple Tests in Sequence**
   ```bash
   go test ./tests/sriov/... -v
   ```
   Expected: All tests complete without hanging on "waiting for operator"

### Monitoring Logs

The improved logging will show:
- "Subscription not found, recreating"
- "Subscription recreated successfully"
- "SriovOperatorConfig ensured"
- "Operator pods found after restoration" with count and iteration

---

## Files Modified

- `tests/sriov/helpers.go`
  - Added `runCommand()` helper
  - Added `manuallyRestoreOperator()` with improved logic

- `tests/sriov/sriov_lifecycle_test.go`
  - Removed duplicate `manuallyRestoreOperator()` function
  - Now uses version from `helpers.go`

- `tests/sriov/sriov_reinstall_test.go`
  - Now uses improved `manuallyRestoreOperator()` from `helpers.go`

---

## Commit Information

**Commit Hash:** 842272ab  
**Title:** feat: Improve SR-IOV operator restoration logic with subscription and config recreation

Changes:
- ✅ Subscription recreation if missing
- ✅ SriovOperatorConfig recreation if missing
- ✅ Extended timeout for operator deployment
- ✅ Better logging for troubleshooting
- ✅ Centralized function in helpers.go

---

## Next Steps

See **Item 3: Document the Findings** for comprehensive documentation to share with the development team.


