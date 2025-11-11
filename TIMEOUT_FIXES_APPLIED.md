# Timeout Fixes Applied - SR-IOV Test Suite

**Date:** 2025-11-11 10:38:56  
**Status:** ✅ Applied - Ready for Testing

---

## Summary

Based on diagnostic investigation (Options B + D), timeout fixes have been applied to improve test suite performance and reliability. The fixes follow **Option 3: Combined Approach** recommended in `TEST_FAILURE_DIAGNOSTIC_REPORT.md`.

---

## Fixes Applied

### Fix 1: NAD Creation Timeout ✅

**File:** `/root/eco-gotests/tests/sriov/sriov_reinstall_test.go`

**Changes:**
- Line 165: Increased `ensureNADExists()` timeout from `30*time.Second` to `120*time.Second`
- Line 298: Increased `ensureNADExists()` timeout from `30*time.Second` to `120*time.Second`

**Rationale:** The diagnostic investigation revealed NAD creation happens within milliseconds, but the operator reconciliation can take up to 100+ seconds due to event queue processing delays. 120 seconds provides sufficient buffer.

**Code Change:**
```go
// Before:
err = ensureNADExists(getAPIClient(), testNetworkName, testNamespace, testNetworkName, 30*time.Second)

// After:
err = ensureNADExists(getAPIClient(), testNetworkName, testNamespace, testNetworkName, 120*time.Second)
```

---

## Fixes Recommended (Not Yet Applied)

### Fix 2: Suite-Wide Timeout (Recommended Next)

To run the full test suite with increased timeout:

```bash
# Current command (times out after 60 minutes):
timeout 3600 ginkgo -v ./tests/sriov/

# Recommended command (120 minutes timeout):
timeout 7200 ginkgo -v ./tests/sriov/
```

**Rationale:** With NAD timeouts increased to 120s per test, a full 21-test suite may exceed 60 minutes.

**Implementation:**
```bash
cd /root/eco-gotests && \
source ~/newlogin.sh 2>/dev/null && \
export GOTOOLCHAIN=auto && \
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0,cx6dxanl244:a2d6:15b3:ens7f0np0" && \
timeout 7200 $(go env GOPATH)/bin/ginkgo -v ./tests/sriov/ 2>&1 | tee /tmp/full_test_run_$(date +%s).log
```

---

## Testing Strategy

### Phase 1: Single Test Diagnostic ✅ (Next)
Run a single test with the applied NAD timeout fixes to verify 120s is sufficient.

```bash
cd /root/eco-gotests && \
source ~/newlogin.sh 2>/dev/null && \
export GOTOOLCHAIN=auto && \
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0,cx6dxanl244:a2d6:15b3:ens7f0np0" && \
timeout 1800 $(go env GOPATH)/bin/ginkgo -v ./tests/sriov/sriov_reinstall_test.go --timeout 15m 2>&1 | tee /tmp/diagnostic_test_$(date +%s).log
```

**Expected Results:**
- Test should PASS with NAD timeout of 120s
- Log should show "NAD ensured to exist" message
- No pod readiness timeout errors

### Phase 2: Full Suite Test (After Phase 1 Success)
Run full test suite with both timeout fixes applied.

```bash
cd /root/eco-gotests && \
source ~/newlogin.sh 2>/dev/null && \
export GOTOOLCHAIN=auto && \
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0,cx6dxanl244:a2d6:15b3:ens7f0np0" && \
timeout 7200 $(go env GOPATH)/bin/ginkgo -v ./tests/sriov/ 2>&1 | tee /tmp/full_test_run_$(date +%s).log
```

**Expected Improvements:**
- All tests should progress beyond NAD timeout point
- More than 5 tests should complete (vs. previous 5/21)
- Pod readiness should be less frequent cause of failure

---

## Verification Checklist

- [x] NAD timeout increased to 120s in sriov_reinstall_test.go
- [x] Orphaned resources cleaned up
- [x] Operator pod verified healthy
- [ ] Single diagnostic test run (Phase 1)
- [ ] Suite timeout increased to 7200s
- [ ] Full test suite run (Phase 2)
- [ ] Results analyzed and documented

---

## Expected Impact

### Current Test Results
- Tests Passed: 1 (setup only)
- Tests Failed: 4
- Tests Timed Out: 1
- Total Executed: 5/21
- Duration: 1h 0m 6s

### Expected After Fixes
- Tests Passed: ✅ Significant increase
- Tests Failed: ⚠️ May still occur (NAD timing, hardware limits)
- Tests Timed Out: ✅ Reduced/eliminated
- Total Executed: ✅ 15-20+ (vs. 5 previously)
- Duration: ~2-2.5 hours (full suite)

### Failure Categories (Expected)
1. NAD timing issues (now with 120s buffer) - Should mostly resolve
2. Hardware resource limitations (single SR-IOV node) - Known issue, not fixable
3. Upstream operator bugs (OCPBUGS-64886) - Will be documented

---

## Diagnostic Data Collection

During test runs, these metrics should be monitored:

```bash
# Track actual NAD creation times
grep -o "elapsed.*[0-9]*\.[0-9]*s" /tmp/full_test_run_*.log | sort | uniq -c

# Count timeout occurrences
grep -c "deadline exceeded\|timeout" /tmp/full_test_run_*.log

# Measure test progress
tail -f /tmp/full_test_run_*.log | grep "PASSED\|FAILED\|TIMEDOUT"
```

---

## Files Modified

1. ✅ `/root/eco-gotests/tests/sriov/sriov_reinstall_test.go` - NAD timeout increased
2. ⏳ (Pending) Increase suite timeout parameter in test execution commands
3. ⏳ (Pending) Document Phase 2 results

---

## Commit Information

**Ready to Commit:**
```bash
git add tests/sriov/sriov_reinstall_test.go TEST_FAILURE_DIAGNOSTIC_REPORT.md TIMEOUT_FIXES_APPLIED.md
git commit -m "Fix: Increase NAD timeout from 30s to 120s to handle operator reconciliation delays

- Changed ensureNADExists() timeout to 120 seconds in sriov_reinstall_test.go
- This addresses timing delays in operator event processing
- Diagnostic report shows operator creates NADs within milliseconds, but reconciliation can take 100+ seconds
- Cleanup of orphaned resources from previous failed test run
- Ready for Phase 1 diagnostic testing"

git push origin main
```

---

## Related Documentation

- `TEST_FAILURE_DIAGNOSTIC_REPORT.md` - Detailed investigation findings
- `UPSTREAM_NAD_CREATION_TIMING_ISSUE.md` - Issue documentation
- `/tmp/full_test_run_*.log` - Test execution logs
- Operator logs - Available via: `oc logs -n openshift-sriov-network-operator deployment/sriov-network-operator`

---

## Status

✅ **Phase 1 (Code Fixes):** COMPLETE
- NAD timeout increased to 120s
- Orphaned resources cleaned up
- Cluster ready for testing

⏳ **Phase 2 (Diagnostic Test):** PENDING
- Single test execution with new timeouts
- Monitor actual NAD creation delays

⏳ **Phase 3 (Full Suite):** PENDING
- Apply suite timeout increase
- Run complete test suite
- Analyze results

---

**Next Action:** Run Phase 1 diagnostic test with applied fixes

