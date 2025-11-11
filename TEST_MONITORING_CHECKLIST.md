# Test Execution Monitoring Checklist

**Test Start Time:** November 11, 2025 (when you restarted tests)  
**Monitoring Interval:** Every 20 minutes  
**Expected Duration:** 1-2 hours  
**Status:** IN PROGRESS

---

## Quick Check Commands

### Check Test Process Status
```bash
ps aux | grep -i ginkgo | grep -v grep
```

### Find Latest Log File
```bash
ls -lht /tmp/full_test_run_*.log | head -1
```

### Check Log File Growth
```bash
du -h /tmp/full_test_run_*.log | tail -1
```

### View Recent Log Entries
```bash
tail -50 /tmp/full_test_run_*.log
```

### Count Test Results So Far
```bash
echo "Passed: $(grep -c '\[PASSED\]' /tmp/full_test_run_*.log || echo '0')"
echo "Failed: $(grep -c '\[FAILED\]' /tmp/full_test_run_*.log || echo '0')"
```

### Check for New Failures
```bash
tail -200 /tmp/full_test_run_*.log | grep -i "STEP:\|FAILED\|\[INFO\]" | tail -20
```

---

## Monitoring Schedule

### Checkpoint 1 (20 min)
- [ ] Test still running?
- [ ] Log size: ___ KB
- [ ] Tests passed: ___
- [ ] Tests failed: ___
- [ ] Current step: ___

### Checkpoint 2 (40 min)
- [ ] Test still running?
- [ ] Log size: ___ KB
- [ ] Tests passed: ___
- [ ] Tests failed: ___
- [ ] Current step: ___

### Checkpoint 3 (60 min)
- [ ] Test still running?
- [ ] Log size: ___ KB
- [ ] Tests passed: ___
- [ ] Tests failed: ___
- [ ] Current step: ___

### Checkpoint 4+ (80 min, 100 min, etc.)
- [ ] Test still running?
- [ ] Log size: ___ KB
- [ ] Tests passed: ___
- [ ] Tests failed: ___
- [ ] Current step: ___

---

## Key Indicators to Look For

### Good Signs ✅
- Ginkgo process still running
- Log file continuously growing
- New steps appearing in logs
- Mixture of PASSED and FAILED (shows progress)
- No hung/stalled processes

### Warning Signs ⚠️
- Log file size not growing for 5+ minutes
- Same step repeated many times
- No new entries for extended period
- Repeated timeout messages

### Issues ❌
- Ginkgo process has stopped
- Log file size static for 10+ minutes
- Only FAILED messages
- Cluster health check fails

---

## What Each Test Phase Looks Like

### Phase 1: Initialization (First 5-10 min)
- Namespace creation
- Resource policy setup
- Operator status checks

### Phase 2: Network Creation (5-15 min)
- SriovNetwork creation
- NAD creation wait
- Policy validation

### Phase 3: Pod Deployment (10-20 min)
- Client/server pod creation
- Pod readiness wait
- Network attachment verification

### Phase 4: Cleanup (5-10 min)
- Resource deletion
- Namespace cleanup
- State verification

---

## Expected Timeline

| Time | Expected Status |
|------|-----------------|
| 0-5 min | Setup/initialization |
| 5-20 min | First test phase |
| 20-40 min | Tests progressing |
| 40-60 min | Mid-test checkpoint |
| 60-90 min | Most tests completed or in progress |
| 90-120 min | Final tests or cleanup |

---

## Troubleshooting Quick Reference

### If test seems hung:
1. Check if ginkgo process exists: `ps aux | grep ginkgo`
2. Check log last entry: `tail -5 /tmp/full_test_run_*.log`
3. Check cluster health: `./tests/sriov/cluster_health_check.sh`

### If many failures:
1. Review last 100 lines: `tail -100 /tmp/full_test_run_*.log`
2. Search for error patterns: `grep -i "error\|failed" /tmp/full_test_run_*.log`
3. Check operator status: `oc get pods -n openshift-sriov-network-operator`

### If test completed:
1. Count final results: `grep -c '\[PASSED\]\|\[FAILED\]' /tmp/full_test_run_*.log`
2. View summary: `tail -100 /tmp/full_test_run_*.log | grep -E "PASSED|FAILED|Summary"`
3. Analyze failures: `grep '\[FAILED\]' /tmp/full_test_run_*.log`

---

## How I'll Monitor

I will check at regular 20-minute intervals:

**Checkpoint 1 (20 min):** Quick status check
- Is test still running?
- Any immediate issues?
- Progress being made?

**Checkpoint 2+ (40, 60, 80, 100+ min):** Detailed status updates
- Test progress percentage
- Pass/fail counts
- Current test being executed
- Any issues detected
- Recommendations

---

## Notes

- Tests may take longer if NAD creation timing issues occur (expected)
- Multiple test files running sequentially
- Some tests marked as [Disruptive] and [Serial]
- Cleanup happens between tests

---

**Last Updated:** November 11, 2025  
**Status:** Monitoring Active

